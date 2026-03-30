package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/denislee/lazylinear/internal/app"
	"github.com/denislee/lazylinear/internal/config"
	"github.com/denislee/lazylinear/internal/linear"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	state := config.LoadState()
	client := linear.NewClient(cfg.APIKey)
	model := app.NewApp(client, state)

	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

	if a, ok := finalModel.(app.App); ok {
		state.LastTeamID = a.CurrentTeamID()
		state.LastFilter = a.CurrentFilter()
		state.CompactMode = a.IsCompact()
		config.SaveState(state)
	}
}
