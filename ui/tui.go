package ui

import (
	"fmt"
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
	app       *tview.Application
	pages     *tview.Pages
	table     *tview.Table
	header    *tview.TextView
	footer    *tview.TextView
	layout    *tview.Flex
	client    *api.Client
	mu        sync.Mutex
	info      models.TournamentInfo
	entries   []models.LeaderboardEntry
	lastFetch time.Time
	err       error
}

// NewApp creates a new TUI application.
func NewApp() *App {
	a := &App{
		app:    tview.NewApplication(),
		client: api.NewClient(),
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

	// Data rows
	for row, entry := range entries {
		r := row + 1
		col := 0

		// Position
		posColor := tcell.ColorWhite
		if entry.Position <= 3 {
			posColor = tcell.ColorGold
		} else if entry.Position <= 10 {
			posColor = tcell.ColorLightGreen
		}
		a.table.SetCell(r, col, tview.NewTableCell(fmt.Sprintf("%d", entry.Position)).
			SetTextColor(posColor).
			SetAlign(tview.AlignCenter))
		col++

		// Player name
		a.table.SetCell(r, col, tview.NewTableCell(entry.Name).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		col++

		// Country
		a.table.SetCell(r, col, tview.NewTableCell(entry.Country).
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
		a.table.SetCell(r, col, tview.NewTableCell(entry.ToPar).
			SetTextColor(toParColor).
			SetAlign(tview.AlignCenter))
		col++

		// Total score
		a.table.SetCell(r, col, tview.NewTableCell(entry.TotalScore).
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
				a.table.SetCell(r, col, tview.NewTableCell(rs).
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
		a.table.SetCell(r, col, tview.NewTableCell(thru).
			SetTextColor(tcell.ColorLightYellow).
			SetAlign(tview.AlignCenter))
		col++

		// Movement
		movColor := tcell.ColorWhite
		if len(entry.Movement) > 0 {
			if entry.Movement[0] == 0xe2 { // ▲ UTF-8 start byte
				movColor = tcell.ColorGreen
			} else if entry.Movement == "-" {
				movColor = tcell.ColorGray
			}
		}
		// Better detection
		if len(entry.Movement) >= 3 && entry.Movement[:3] == "▲" {
			movColor = tcell.ColorGreen
		} else if len(entry.Movement) >= 3 && entry.Movement[:3] == "▼" {
			movColor = tcell.ColorRed
		}
		a.table.SetCell(r, col, tview.NewTableCell(entry.Movement).
			SetTextColor(movColor).
			SetAlign(tview.AlignCenter))
	}

	a.table.ScrollToBeginning()
}

// renderFooter updates the footer with status info.
func (a *App) renderFooter() {
	a.mu.Lock()
	lastFetch := a.lastFetch
	a.mu.Unlock()

	footerText := fmt.Sprintf(
		"[darkgray]Last updated: %s  |  [green]r[darkgray] refresh  |  [green]q/Esc[darkgray] quit  |  [green]↑↓[darkgray] scroll",
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
