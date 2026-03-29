package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var (
	// Border styles
	FocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4"))

	BlurredBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444"))

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888"))

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	// Modal style
	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2).
			Width(60)
)

// StatusColor returns the color for a workflow state type.
func StatusColor(stateType string) color.Color {
	switch stateType {
	case "triage":
		return lipgloss.Color("#FFCC00") // yellow
	case "backlog":
		return lipgloss.Color("#888888") // gray
	case "unstarted":
		return lipgloss.Color("#CCCCCC") // light gray
	case "started":
		return lipgloss.Color("#FF8800") // orange
	case "completed":
		return lipgloss.Color("#00CC66") // green
	case "canceled":
		return lipgloss.Color("#CC0000") // red
	default:
		return lipgloss.Color("#888888") // gray fallback
	}
}

// StatusStyle returns a styled lipgloss style for a workflow state type.
func StatusStyle(stateType string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(StatusColor(stateType))
}
