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

	client := linear.NewClient(cfg.APIKey)
	model := app.NewApp(client)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
