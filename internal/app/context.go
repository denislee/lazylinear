package app

import "github.com/denislee/lazylinear/internal/linear"

// AppContext holds shared application state accessible by all panels.
type AppContext struct {
	Client      *linear.Client
	CurrentUser    *linear.User
	CurrentTeam    *linear.Team
	CurrentProjects []linear.Project
	Teams          []linear.Team
	Width       int
	Height      int
}
