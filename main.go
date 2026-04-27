package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/denislee/lazylinear/internal/app"
	"github.com/denislee/lazylinear/internal/config"
	"github.com/denislee/lazylinear/internal/linear"
)

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	// Ensure a panic in the TUI never leaves the terminal in a broken state
	// without context about what happened.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "lazylinear: panic: %v\n\n%s\n", r, debug.Stack())
			exitCode = 2
		}
	}()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	state := config.LoadState()
	client := linear.NewClient(cfg.APIKey)
	model := app.NewApp(client, state)

	p := tea.NewProgram(model)
	finalModel, runErr := p.Run()

	// Persist whatever state we can, even if Run errored — the user's last
	// selections are more valuable than the error path being pristine.
	if a, ok := finalModel.(app.App); ok {
		state.LastTeamID = a.CurrentTeamID()
		state.LastFilter = a.CurrentFilter()
		state.CompactMode = a.IsCompact()
		if err := config.SaveState(state); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save state: %v\n", err)
		}
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", runErr)
		return 1
	}
	return 0
}
