package statusbar

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	appmsg "github.com/denislee/lazylinear/internal/msg"
)

var (
	barStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#333"))

	contextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#333")).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Background(lipgloss.Color("#333"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Background(lipgloss.Color("#333")).
			Bold(true)
)

// Model is the status bar panel.
type Model struct {
	width    int
	teamName string
	filter   string
	hints    string
	errMsg   string
}

// New creates a new status bar model.
func New() Model {
	return Model{
		hints: "tab: switch panel | q: quit | ?: help",
	}
}

// SetSize updates the status bar width.
func (m *Model) SetSize(width int) {
	m.width = width
}

// SetTeam updates the displayed team name.
func (m *Model) SetTeam(name string) {
	m.teamName = name
}

// SetFilter updates the displayed filter.
func (m *Model) SetFilter(filter string) {
	m.filter = filter
}

// SetError sets an error message to display.
func (m *Model) SetError(err string) {
	m.errMsg = err
}

// ClearError clears the error message.
func (m *Model) ClearError() {
	m.errMsg = ""
}

// SetHints updates the key hint text.
func (m *Model) SetHints(hints string) {
	m.hints = hints
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case appmsg.TeamSelectedMsg:
		m.teamName = msg.Team.Name
		m.errMsg = ""
	case appmsg.ErrorMsg:
		m.errMsg = msg.Err.Error()
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.width <= 0 {
		return tea.NewView("")
	}

	// Build left side: context breadcrumb.
	var left string
	if m.errMsg != "" {
		left = errorStyle.Render(fmt.Sprintf(" ERROR: %s", m.errMsg))
	} else if m.teamName != "" {
		breadcrumb := m.teamName
		if m.filter != "" {
			breadcrumb += " > " + m.filter
		}
		left = contextStyle.Render(" " + breadcrumb)
	} else {
		left = contextStyle.Render(" lazylinear")
	}

	// Build right side: key hints.
	right := hintStyle.Render(m.hints + " ")

	// Calculate padding between left and right.
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	pad := m.width - leftLen - rightLen
	if pad < 0 {
		pad = 0
	}

	bar := left + barStyle.Render(strings.Repeat(" ", pad)) + right
	return tea.NewView(bar)
}
