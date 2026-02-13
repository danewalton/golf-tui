package main

import (
	"fmt"
	"os"

	"github.com/danewalton/sports-tui/ui"
)

func main() {
	app := ui.NewApp()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
