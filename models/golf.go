package models

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ESPNResponse is the top-level response from the ESPN Golf API.
type ESPNResponse struct {
	Leagues []League `json:"leagues"`
	Events  []Event  `json:"events"`
}

// League contains league metadata.
type League struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Season       Season `json:"season"`
}

// Season contains season info.
type Season struct {
	Year int        `json:"year"`
	Type SeasonType `json:"type"`
	Name string     `json:"name"`
}

// SeasonType describes the type of season.
type SeasonType struct {
	ID           string `json:"id"`
	Type         int    `json:"type"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

// Event represents a single tournament.
type Event struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	ShortName    string        `json:"shortName"`
	Competitions []Competition `json:"competitions"`
	Status       EventStatus   `json:"status"`
}

// EventStatus contains the status of the event.
type EventStatus struct {
	Type StatusType `json:"type"`
}

// StatusType describes the type of event status.
type StatusType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Completed   bool   `json:"completed"`
	Description string `json:"description"`
}

// Competition represents a competition within an event (usually one per event).
type Competition struct {
	ID          string       `json:"id"`
	Competitors []Competitor `json:"competitors"`
	Status      CompStatus   `json:"status"`
	Venue       Venue        `json:"venue"`
}

// CompStatus contains competition status details.
type CompStatus struct {
	Period       int        `json:"period"`
	Type         StatusType `json:"type"`
	DisplayClock string     `json:"displayClock"`
}

// Venue contains venue information.
type Venue struct {
	FullName string  `json:"fullName"`
	Address  Address `json:"address"`
}

// Address contains location details.
type Address struct {
	City  string `json:"city"`
	State string `json:"state"`
}

// Competitor represents a golfer in the competition.
type Competitor struct {
	ID         string            `json:"id"`
	UID        string            `json:"uid"`
	Status     *CompetitorStatus `json:"status,omitempty"`
	Score      string            `json:"score"`
	Athlete    Athlete           `json:"athlete"`
	Order      int               `json:"order"`
	SortOrder  int               `json:"sortOrder"`
	Statistics []Statistic       `json:"statistics"`
	Linescores []Linescore       `json:"linescores"`
	Movement   int               `json:"movement"`
}

// CompetitorStatus describes cut status, etc.
type CompetitorStatus struct {
	Period       int        `json:"period"`
	Type         StatusType `json:"type"`
	DisplayValue string     `json:"displayValue"`
}

// Athlete contains golfer info.
type Athlete struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	ShortName   string `json:"shortName"`
	Flag        Flag   `json:"flag"`
	Headshot    string `json:"headshot"`
}

// Flag contains country info.
type Flag struct {
	Href string `json:"href"`
	Alt  string `json:"alt"`
}

// Statistic holds a stat value (e.g., "toPar").
type Statistic struct {
	Name             string  `json:"name"`
	DisplayName      string  `json:"displayName"`
	ShortDisplayName string  `json:"shortDisplayName"`
	Description      string  `json:"description"`
	Abbreviation     string  `json:"abbreviation"`
	Value            float64 `json:"value"`
	DisplayValue     string  `json:"displayValue"`
}

// FlexString handles JSON values that can be either a string or a number.
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	// Try number
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		// Format as integer if whole number
		if n == float64(int64(n)) {
			*f = FlexString(strconv.FormatInt(int64(n), 10))
		} else {
			*f = FlexString(strconv.FormatFloat(n, 'f', -1, 64))
		}
		return nil
	}
	*f = FlexString(string(data))
	return nil
}

func (f FlexString) String() string {
	return string(f)
}

// Linescore represents one round score.
type Linescore struct {
	Period       int             `json:"period"`
	Value        FlexString      `json:"value"`
	DisplayValue string          `json:"displayValue"`
	Linescores   []HoleLinescore `json:"linescores"`
	Statistics   json.RawMessage `json:"statistics,omitempty"`
}

// ScoreType describes the score relative to par for a hole.
type ScoreType struct {
	DisplayValue string `json:"displayValue"`
}

// HoleLinescore represents a single hole score within a round.
type HoleLinescore struct {
	Value        FlexString `json:"value"`
	DisplayValue string     `json:"displayValue"`
	Period       int        `json:"period"`
	ScoreType    ScoreType  `json:"scoreType"`
}

// RoundScorecard holds hole-by-hole scores for a single round.
type RoundScorecard struct {
	Round      int
	Scores     [18]string // index 0 = hole 1
	ScoreToPar [18]string // e.g. "-1" birdie, "E" par, "+1" bogey
	Total      string
}

// LeaderboardEntry is a simplified view of a competitor for display.
type LeaderboardEntry struct {
	Position     int
	Name         string
	Country      string
	ToPar        string
	TotalScore   string
	Round1       string
	Round2       string
	Round3       string
	Round4       string
	CurrentRound string
	Thru         string
	Movement     string
	Status       string
	RoundScores  []RoundScorecard
}

// EventSummary holds a brief summary of a tournament for display in the picker.
type EventSummary struct {
	Index  int
	Name   string
	State  string // "pre", "in", "post"
	Status string // "Upcoming", "In Progress", "Completed"
}

// TournamentInfo holds tournament metadata for display.
type TournamentInfo struct {
	Name       string
	Venue      string
	Location   string
	Status     string
	Round      int
	EventState string
}

// FormatToPar formats a to-par value for display.
func FormatToPar(val string) string {
	if val == "E" || val == "0" {
		return "E"
	}
	return val
}

// FormatMovement formats movement as an arrow indicator.
func FormatMovement(m int) string {
	if m > 0 {
		return fmt.Sprintf("▲%d", m)
	} else if m < 0 {
		return fmt.Sprintf("▼%d", -m)
	}
	return "-"
}
