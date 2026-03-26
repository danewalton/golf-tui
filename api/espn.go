package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/danewalton/sports-tui/models"
)

const (
	espnGolfScoreboardURL = "https://site.api.espn.com/apis/site/v2/sports/golf/pga/scoreboard"
)

// Client handles API requests to ESPN.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchScoreboard retrieves the PGA Tour scoreboard from ESPN.
// date is an optional YYYYMMDD string; empty means the current week.
func (c *Client) FetchScoreboard(date string) (*models.ESPNResponse, error) {
	url := espnGolfScoreboardURL
	if date != "" {
		url += "?dates=" + date
	}
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scoreboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var data models.ESPNResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &data, nil
}

// GetEventSummaries returns a brief summary of every event in the response,
// suitable for populating the tournament picker.
func GetEventSummaries(data *models.ESPNResponse) []models.EventSummary {
	summaries := make([]models.EventSummary, 0, len(data.Events))
	for i, event := range data.Events {
		status := ""
		switch event.Status.Type.State {
		case "pre":
			status = "Upcoming"
		case "in":
			status = "In Progress"
		case "post":
			status = "Completed"
		default:
			status = event.Status.Type.Description
		}
		summaries = append(summaries, models.EventSummary{
			Index:  i,
			Name:   event.Name,
			State:  event.Status.Type.State,
			Status: status,
		})
	}
	return summaries
}

// GetTournamentInfo extracts tournament metadata from the API response.
// eventIdx selects which event to read (0-based); if out of range the first is used.
func GetTournamentInfo(data *models.ESPNResponse, eventIdx int) models.TournamentInfo {
	info := models.TournamentInfo{
		Name:       "No Active Tournament",
		Venue:      "",
		Location:   "",
		Status:     "Unknown",
		Round:      0,
		EventState: "unknown",
	}

	if len(data.Events) == 0 {
		return info
	}
	if eventIdx < 0 || eventIdx >= len(data.Events) {
		eventIdx = 0
	}

	event := data.Events[eventIdx]
	info.Name = event.Name
	info.EventState = event.Status.Type.State

	switch event.Status.Type.State {
	case "pre":
		info.Status = "Upcoming"
	case "in":
		info.Status = "In Progress"
	case "post":
		info.Status = "Completed"
	default:
		info.Status = event.Status.Type.Description
	}

	if len(event.Competitions) > 0 {
		comp := event.Competitions[0]
		info.Round = comp.Status.Period

		if comp.Venue.FullName != "" {
			info.Venue = comp.Venue.FullName
		}
		if comp.Venue.Address.City != "" {
			info.Location = comp.Venue.Address.City
			if comp.Venue.Address.State != "" {
				info.Location += ", " + comp.Venue.Address.State
			}
		}
	}

	return info
}

// GetLeaderboard extracts and sorts the leaderboard from the API response.
// eventIdx selects which event to read (0-based); if out of range the first is used.
func GetLeaderboard(data *models.ESPNResponse, eventIdx int) []models.LeaderboardEntry {
	entries := []models.LeaderboardEntry{}

	if len(data.Events) == 0 {
		return entries
	}
	if eventIdx < 0 || eventIdx >= len(data.Events) {
		eventIdx = 0
	}

	event := data.Events[eventIdx]
	if len(event.Competitions) == 0 {
		return entries
	}

	comp := event.Competitions[0]

	// Sort competitors by order (ESPN provides them pre-sorted, but use Order/SortOrder as fallback)
	competitors := make([]models.Competitor, len(comp.Competitors))
	copy(competitors, comp.Competitors)
	sort.Slice(competitors, func(i, j int) bool {
		oi, oj := competitors[i].Order, competitors[j].Order
		if oi == 0 {
			oi = competitors[i].SortOrder
		}
		if oj == 0 {
			oj = competitors[j].SortOrder
		}
		if oi != oj {
			return oi < oj
		}
		return i < j // stable fallback
	})

	currentRound := comp.Status.Period

	for i, c := range competitors {
		entry := models.LeaderboardEntry{
			Position:   i + 1,
			Name:       c.Athlete.DisplayName,
			Country:    c.Athlete.Flag.Alt,
			TotalScore: c.Score,
			Movement:   models.FormatMovement(c.Movement),
		}

		// Extract to-par from statistics
		for _, stat := range c.Statistics {
			if stat.Name == "scoreToPar" || stat.Name == "toPar" {
				entry.ToPar = models.FormatToPar(stat.DisplayValue)
				break
			}
		}
		if entry.ToPar == "" {
			// Use score as to-par (ESPN returns e.g. "-15")
			entry.ToPar = models.FormatToPar(c.Score)
		}

		// Extract round scores and thru from linescores
		for _, ls := range c.Linescores {
			// Use displayValue for round score (e.g. "-7") if available,
			// fall back to value (stroke total e.g. "65")
			roundScore := ls.DisplayValue
			if roundScore == "" && ls.Value.String() != "" {
				roundScore = ls.Value.String()
			}

			switch ls.Period {
			case 1:
				entry.Round1 = roundScore
			case 2:
				entry.Round2 = roundScore
			case 3:
				entry.Round3 = roundScore
			case 4:
				entry.Round4 = roundScore
			}

			// Derive "thru" from the current round's hole-by-hole linescores
			if ls.Period == currentRound {
				holesPlayed := len(ls.Linescores)
				if holesPlayed == 18 {
					entry.Thru = "F"
				} else if holesPlayed > 0 {
					entry.Thru = fmt.Sprintf("%d", holesPlayed)
				}
			}

			// Collect hole-by-hole scorecard data
			if len(ls.Linescores) > 0 {
				sc := models.RoundScorecard{
					Round: ls.Period,
					Total: ls.Value.String(),
				}
				for _, hs := range ls.Linescores {
					if hs.Period >= 1 && hs.Period <= 18 {
						sc.Scores[hs.Period-1] = hs.Value.String()
						sc.ScoreToPar[hs.Period-1] = hs.ScoreType.DisplayValue
					}
				}
				entry.RoundScores = append(entry.RoundScores, sc)
			}
		}

		// If no thru derived yet, check if they have all rounds completed
		if entry.Thru == "" {
			// Check if the most recent round with data is complete
			for ri := len(c.Linescores) - 1; ri >= 0; ri-- {
				ls := c.Linescores[ri]
				if len(ls.Linescores) == 18 {
					entry.Thru = "F"
					break
				} else if len(ls.Linescores) > 0 {
					entry.Thru = fmt.Sprintf("%d", len(ls.Linescores))
					break
				}
			}
		}

		entry.CurrentRound = fmt.Sprintf("R%d", currentRound)

		// Status from competitor if available
		if c.Status != nil {
			if c.Status.DisplayValue != "" {
				entry.Status = c.Status.DisplayValue
			}
			if c.Status.Type.Description != "" {
				entry.Status = c.Status.Type.Description
			}
		}

		entries = append(entries, entry)
	}

	return entries
}
