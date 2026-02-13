# Sports TUI App

A terminal user interface (TUI) application that displays live PGA Tour tournament leaderboards directly in your terminal.

Built with Go and [`tview`](https://github.com/rivo/tview). Data is sourced from the ESPN API.

## Features

- **Live leaderboard** — player positions, to-par scores, round-by-round scores, and movement indicators
- **Tournament info** — event name, venue, location, round, and status
- **Auto-refresh** — data refreshes every 60 seconds
- **Keyboard controls** — scroll with arrow keys, `r` to refresh, `q`/`Esc` to quit
- **Color-coded** — under par (red), even (yellow), over par (blue), top positions highlighted

## Requirements

- Go 1.21+

## Installation

```bash
git clone https://github.com/danewalton/sports-tui.git
cd sports-tui
go build -o sports-tui .
```

## Usage

```bash
./sports-tui
```

### Key Bindings

| Key       | Action            |
|-----------|-------------------|
| `↑` / `↓` | Scroll leaderboard |
| `r`       | Refresh data      |
| `q` / `Esc` | Quit             |

## Project Structure

```
sports-tui/
├── main.go          # Entry point
├── api/
│   └── espn.go      # ESPN API client & data extraction
├── models/
│   └── golf.go      # Data models & formatting helpers
└── ui/
    └── tui.go       # TUI components & rendering
```
