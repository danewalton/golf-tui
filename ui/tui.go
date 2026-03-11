package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danewalton/sports-tui/api"
	"github.com/danewalton/sports-tui/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	refreshInterval = 60 * time.Second
)

// App holds all TUI components and state.
type App struct {
	app        *tview.Application
	pages      *tview.Pages
	table      *tview.Table
	header     *tview.TextView
	footer     *tview.TextView
	layout     *tview.Flex
	client     *api.Client
	mu         sync.Mutex
	info       models.TournamentInfo
	entries    []models.LeaderboardEntry
	lastFetch  time.Time
	err        error
	expanded   map[int]bool // which entry indices are expanded (scorecard visible)
	rowToEntry map[int]int  // table row -> entry index (-1 for scorecard rows)
}

// NewApp creates a new TUI application.
func NewApp() *App {
	a := &App{
		app:        tview.NewApplication(),
		client:     api.NewClient(),
		expanded:   make(map[int]bool),
		rowToEntry: make(map[int]int),
	}
	a.buildUI()
	return a
}

// buildUI constructs the TUI layout.
func (a *App) buildUI() {
	// Header - tournament info
	a.header = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	a.header.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGreen).
		SetTitle(" 🏌 PGA Tour ").
		SetTitleColor(tcell.ColorGreen).
		SetBorderPadding(0, 0, 1, 1)

	// Leaderboard table
	a.table = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(tview.Borders.Vertical)
	a.table.SetBorder(true).
		SetBorderColor(tcell.ColorDarkGreen).
		SetTitle(" Leaderboard ").
		SetTitleColor(tcell.ColorGreen).
		SetBorderPadding(0, 0, 1, 1)

	// Toggle scorecard on Enter
	a.table.SetSelectedFunc(func(row, column int) {
		entryIdx, ok := a.rowToEntry[row]
		if !ok || entryIdx < 0 {
			return
		}
		if a.expanded[entryIdx] {
			delete(a.expanded, entryIdx)
		} else {
			a.expanded[entryIdx] = true
		}
		a.renderTable()
		// Re-select the player row after re-render
		for r, idx := range a.rowToEntry {
			if idx == entryIdx {
				a.table.Select(r, 0)
				break
			}
		}
	})

	// Footer - controls & status
	a.footer = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	a.footer.SetBorder(false).
		SetBorderPadding(0, 0, 0, 0)

	// Main layout
	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 5, 0, false).
		AddItem(a.table, 0, 1, true).
		AddItem(a.footer, 1, 0, false)

	// Pages for potential modal dialogs
	a.pages = tview.NewPages().
		AddPage("main", a.layout, true, true)

	// Key bindings
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				a.app.Stop()
				return nil
			case 'r':
				go a.refreshData()
				return nil
			}
		case tcell.KeyEsc:
			a.app.Stop()
			return nil
		}
		return event
	})
}

// Run starts the TUI application.
func (a *App) Run() error {
	// Show a loading state immediately
	a.header.SetText("[bold][green]PGA Tour Leaderboard\n[yellow]Loading...")
	a.footer.SetText("[darkgray]Fetching data...  |  [green]q/Esc[darkgray] quit")

	// Fetch data asynchronously so the TUI renders immediately
	go func() {
		a.refreshData()
		a.autoRefresh()
	}()

	return a.app.SetRoot(a.pages, true).EnableMouse(true).Run()
}

// autoRefresh periodically fetches new data.
func (a *App) autoRefresh() {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for range ticker.C {
		a.refreshData()
	}
}

// refreshData fetches data and updates the UI.
func (a *App) refreshData() {
	data, err := a.client.FetchScoreboard()

	a.mu.Lock()
	a.lastFetch = time.Now()
	if err != nil {
		a.err = err
		a.mu.Unlock()
		a.app.QueueUpdateDraw(func() {
			a.renderError()
		})
		return
	}
	a.err = nil
	a.info = api.GetTournamentInfo(data)
	a.entries = api.GetLeaderboard(data)
	a.mu.Unlock()

	a.app.QueueUpdateDraw(func() {
		a.renderHeader()
		a.renderTable()
		a.renderFooter()
	})
}

// renderHeader updates the header with tournament info.
func (a *App) renderHeader() {
	a.mu.Lock()
	info := a.info
	a.mu.Unlock()

	var venueStr string
	if info.Venue != "" {
		venueStr = fmt.Sprintf("  |  [white]%s", info.Venue)
	}
	var locationStr string
	if info.Location != "" {
		locationStr = fmt.Sprintf("  |  [white]%s", info.Location)
	}

	statusColor := "yellow"
	switch info.EventState {
	case "in":
		statusColor = "lime"
	case "post":
		statusColor = "gray"
	case "pre":
		statusColor = "cyan"
	}

	headerText := fmt.Sprintf(
		"[bold][green]%s[white]\n[%s]%s[white]%s%s",
		info.Name,
		statusColor, info.Status,
		venueStr,
		locationStr,
	)

	if info.Round > 0 {
		headerText += fmt.Sprintf("  |  [yellow]Round %d", info.Round)
	}

	a.header.SetText(headerText)
}

// renderTable updates the leaderboard table.
func (a *App) renderTable() {
	a.mu.Lock()
	entries := a.entries
	info := a.info
	a.mu.Unlock()

	a.table.Clear()
	a.rowToEntry = make(map[int]int)

	if len(entries) == 0 {
		a.table.SetCell(0, 0, tview.NewTableCell("[yellow]No leaderboard data available. The tournament may not have started yet.").
			SetExpansion(1).
			SetAlign(tview.AlignCenter))
		return
	}

	// Determine which columns to show based on tournament state
	showRounds := info.EventState == "in" || info.EventState == "post"

	// Header row
	headers := []string{"POS", "PLAYER", "COUNTRY", "TO PAR", "TOTAL"}
	if showRounds {
		headers = append(headers, "R1", "R2", "R3", "R4")
	}
	headers = append(headers, "THRU", "MOV")
	numCols := len(headers)

	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorGreen).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetAlign(tview.AlignCenter)
		if h == "PLAYER" {
			cell.SetAlign(tview.AlignLeft).SetExpansion(1)
		}
		a.table.SetCell(0, col, cell)
	}

	// Data rows (with interleaved scorecard rows for expanded players)
	tableRow := 1
	for i, entry := range entries {
		a.rowToEntry[tableRow] = i
		col := 0

		// Position
		posColor := tcell.ColorWhite
		if entry.Position <= 3 {
			posColor = tcell.ColorGold
		} else if entry.Position <= 10 {
			posColor = tcell.ColorLightGreen
		}
		a.table.SetCell(tableRow, col, tview.NewTableCell(fmt.Sprintf("%d", entry.Position)).
			SetTextColor(posColor).
			SetAlign(tview.AlignCenter))
		col++

		// Player name (add indicator if expandable)
		nameText := entry.Name
		if a.expanded[i] {
			nameText = "▾ " + nameText
		} else if len(entry.RoundScores) > 0 {
			nameText = "▸ " + nameText
		}
		a.table.SetCell(tableRow, col, tview.NewTableCell(nameText).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		col++

		// Country
		a.table.SetCell(tableRow, col, tview.NewTableCell(entry.Country).
			SetTextColor(tcell.ColorLightGray).
			SetAlign(tview.AlignCenter))
		col++

		// To Par
		toParColor := tcell.ColorWhite
		if entry.ToPar == "E" {
			toParColor = tcell.ColorYellow
		} else if len(entry.ToPar) > 0 && entry.ToPar[0] == '-' {
			toParColor = tcell.ColorRed
		} else if len(entry.ToPar) > 0 && entry.ToPar[0] == '+' {
			toParColor = tcell.ColorDeepSkyBlue
		}
		a.table.SetCell(tableRow, col, tview.NewTableCell(entry.ToPar).
			SetTextColor(toParColor).
			SetAlign(tview.AlignCenter))
		col++

		// Total score
		a.table.SetCell(tableRow, col, tview.NewTableCell(entry.TotalScore).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignCenter))
		col++

		// Round scores
		if showRounds {
			rounds := []string{entry.Round1, entry.Round2, entry.Round3, entry.Round4}
			for _, rs := range rounds {
				if rs == "" {
					rs = "-"
				}
				a.table.SetCell(tableRow, col, tview.NewTableCell(rs).
					SetTextColor(tcell.ColorLightGray).
					SetAlign(tview.AlignCenter))
				col++
			}
		}

		// Thru
		thru := entry.Thru
		if thru == "" {
			thru = "-"
		}
		a.table.SetCell(tableRow, col, tview.NewTableCell(thru).
			SetTextColor(tcell.ColorLightYellow).
			SetAlign(tview.AlignCenter))
		col++

		// Movement
		movColor := tcell.ColorWhite
		if len(entry.Movement) >= 3 && entry.Movement[:3] == "▲" {
			movColor = tcell.ColorGreen
		} else if len(entry.Movement) >= 3 && entry.Movement[:3] == "▼" {
			movColor = tcell.ColorRed
		} else if entry.Movement == "-" {
			movColor = tcell.ColorGray
		}
		a.table.SetCell(tableRow, col, tview.NewTableCell(entry.Movement).
			SetTextColor(movColor).
			SetAlign(tview.AlignCenter))

		tableRow++

		// Render scorecard rows if this player is expanded
		if a.expanded[i] && len(entry.RoundScores) > 0 {
			// Hole header row
			a.rowToEntry[tableRow] = -1
			holeHeader := "        "
			for h := 1; h <= 9; h++ {
				holeHeader += fmt.Sprintf("%4d", h)
			}
			holeHeader += "   OUT"
			for h := 10; h <= 18; h++ {
				holeHeader += fmt.Sprintf("%4d", h)
			}
			holeHeader += "    IN  TOT"
			a.table.SetCell(tableRow, 0, tview.NewTableCell("").SetSelectable(false))
			a.table.SetCell(tableRow, 1, tview.NewTableCell(holeHeader).
				SetTextColor(tcell.ColorDarkCyan).
				SetExpansion(1).
				SetSelectable(false))
			for c := 2; c < numCols; c++ {
				a.table.SetCell(tableRow, c, tview.NewTableCell("").SetSelectable(false))
			}
			tableRow++

			// One row per round
			for _, sc := range entry.RoundScores {
				a.rowToEntry[tableRow] = -1
				line := fmt.Sprintf("    R%-3d", sc.Round)
				front9 := 0
				frontCount := 0
				for h := 0; h < 9; h++ {
					line += formatHoleScore(sc.Scores[h], sc.ScoreToPar[h])
					if v, err := strconv.Atoi(sc.Scores[h]); err == nil {
						front9 += v
						frontCount++
					}
				}
				if frontCount == 9 {
					line += fmt.Sprintf("  %4d", front9)
				} else {
					line += "     -"
				}
				back9 := 0
				backCount := 0
				for h := 9; h < 18; h++ {
					line += formatHoleScore(sc.Scores[h], sc.ScoreToPar[h])
					if v, err := strconv.Atoi(sc.Scores[h]); err == nil {
						back9 += v
						backCount++
					}
				}
				if backCount == 9 {
					line += fmt.Sprintf("  %4d", back9)
				} else {
					line += "     -"
				}
				total := sc.Total
				if total == "" {
					total = "-"
				}
				line += fmt.Sprintf("  %3s", total)

				a.table.SetCell(tableRow, 0, tview.NewTableCell("").SetSelectable(false))
				a.table.SetCell(tableRow, 1, tview.NewTableCell(line).
					SetTextColor(tcell.ColorLightSlateGray).
					SetExpansion(1).
					SetSelectable(false))
				for c := 2; c < numCols; c++ {
					a.table.SetCell(tableRow, c, tview.NewTableCell("").SetSelectable(false))
				}
				tableRow++
			}

			// Legend row
			a.rowToEntry[tableRow] = -1
			legend := "        [red]◎[-] Eagle   [red]○[-] Birdie   · Par   [navy]□[-] Bogey   [navy]■[-] Dbl Bogey+"
			a.table.SetCell(tableRow, 0, tview.NewTableCell("").SetSelectable(false))
			a.table.SetCell(tableRow, 1, tview.NewTableCell(legend).
				SetExpansion(1).
				SetSelectable(false))
			for c := 2; c < numCols; c++ {
				a.table.SetCell(tableRow, c, tview.NewTableCell("").SetSelectable(false))
			}
			tableRow++
		}
	}

	a.table.ScrollToBeginning()
}

// renderFooter updates the footer with status info.
func (a *App) renderFooter() {
	a.mu.Lock()
	lastFetch := a.lastFetch
	a.mu.Unlock()

	footerText := fmt.Sprintf(
		"[darkgray]Last updated: %s  |  [green]r[darkgray] refresh  |  [green]q/Esc[darkgray] quit  |  [green]↑↓[darkgray] scroll  |  [green]Enter[darkgray] scorecard",
		lastFetch.Format("3:04:05 PM"),
	)
	a.footer.SetText(footerText)
}

// renderError shows an error message.
func (a *App) renderError() {
	a.mu.Lock()
	err := a.err
	lastFetch := a.lastFetch
	a.mu.Unlock()

	a.header.SetText(fmt.Sprintf("[bold][green]PGA Tour Leaderboard\n[red]Error: %v", err))
	a.footer.SetText(fmt.Sprintf(
		"[darkgray]Last attempt: %s  |  [green]r[darkgray] retry  |  [green]q/Esc[darkgray] quit",
		lastFetch.Format("3:04:05 PM"),
	))
}

// formatHoleScore returns a formatted hole score decorated with
// colored unicode indicators for the score relative to par:
//
//	◎3  eagle or better (red double circle)
//	○3  birdie (red circle)
//	·4  par (dot)
//	□5  bogey (navy square)
//	■6  double-bogey or worse (navy filled square)
func formatHoleScore(score, toPar string) string {
	if score == "" {
		return "   -"
	}
	var prefix string
	switch {
	case toPar == "E" || toPar == "":
		prefix = "·"
	case strings.HasPrefix(toPar, "-"):
		n, _ := strconv.Atoi(toPar)
		if n <= -2 {
			prefix = "[red]◎[-]" // eagle or better
		} else {
			prefix = "[red]○[-]" // birdie
		}
	case strings.HasPrefix(toPar, "+"):
		n, _ := strconv.Atoi(toPar)
		if n >= 2 {
			prefix = "[navy]■[-]" // double-bogey+
		} else {
			prefix = "[navy]□[-]" // bogey
		}
	default:
		prefix = "·"
	}
	return fmt.Sprintf(" %s%s", prefix, score)
}
