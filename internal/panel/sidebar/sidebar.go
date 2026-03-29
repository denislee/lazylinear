package sidebar

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/theme"
)

// Filter categories shown below the team list.
var filterCategories = []string{
	"My Issues",
	"All Issues",
	"Active",
	"Backlog",
}

// SectionTeams is the teams section of the sidebar.
const (
	SectionTeams   = 0
	SectionFilters = 1
)

// Model is the sidebar panel.
type Model struct {
	teams          []linear.Team
	selectedTeam   int
	cursor         int
	section        int // 0 = teams, 1 = filters
	filterCursor   int
	selectedFilter int // currently active filter index
	width          int
	height         int
	focused        bool
	loading        bool
	loadFailed     bool
	spinner        spinner.Model
}

// New creates a new sidebar model.
func New() Model {
	s := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))),
	)
	return Model{
		section:      SectionTeams,
		filterCursor: 0,
		loading:      true,
		spinner:      s,
	}
}

// SetSize updates the sidebar dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets the focus state of the sidebar.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// Focused returns whether the sidebar is focused.
func (m Model) Focused() bool {
	return m.focused
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case appmsg.TeamsLoadedMsg:
		m.teams = msg.Teams
		m.cursor = 0
		m.section = SectionTeams
		m.loading = false
		m.loadFailed = false

	case appmsg.ErrorMsg:
		// If we were loading teams and got an error, mark load as failed.
		if m.loading {
			m.loading = false
			m.loadFailed = true
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	totalItems := m.totalItems()
	if totalItems == 0 {
		return m, nil
	}

	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)

	case "k", "up":
		m.moveCursor(-1)

	case "g":
		// Go to top.
		m.section = SectionTeams
		m.cursor = 0
		m.filterCursor = 0

	case "G":
		// Go to bottom.
		if len(filterCategories) > 0 {
			m.section = SectionFilters
			m.filterCursor = len(filterCategories) - 1
		} else if len(m.teams) > 0 {
			m.section = SectionTeams
			m.cursor = len(m.teams) - 1
		}

	case "enter", "l":
		return m.selectItem()
	}

	return m, nil
}

func (m *Model) moveCursor(delta int) {
	if m.section == SectionTeams {
		m.cursor += delta
		if m.cursor < 0 {
			m.cursor = 0
		} else if m.cursor >= len(m.teams) {
			// Move to filters section.
			if len(filterCategories) > 0 {
				m.section = SectionFilters
				m.filterCursor = m.cursor - len(m.teams)
				if m.filterCursor >= len(filterCategories) {
					m.filterCursor = len(filterCategories) - 1
				}
				m.cursor = len(m.teams) - 1
			} else {
				m.cursor = len(m.teams) - 1
			}
		}
	} else {
		m.filterCursor += delta
		if m.filterCursor < 0 {
			// Move back to teams section.
			if len(m.teams) > 0 {
				m.section = SectionTeams
				m.cursor = len(m.teams) - 1
			} else {
				m.filterCursor = 0
			}
		} else if m.filterCursor >= len(filterCategories) {
			m.filterCursor = len(filterCategories) - 1
		}
	}
}

func (m Model) selectItem() (tea.Model, tea.Cmd) {
	if m.section == SectionTeams && m.cursor < len(m.teams) {
		m.selectedTeam = m.cursor
		return m, func() tea.Msg {
			return appmsg.TeamSelectedMsg{Team: m.teams[m.cursor]}
		}
	}
	if m.section == SectionFilters && m.filterCursor < len(filterCategories) {
		m.selectedFilter = m.filterCursor
		filter := filterCategories[m.filterCursor]
		return m, func() tea.Msg {
			return appmsg.FilterSelectedMsg{Filter: filter}
		}
	}
	return m, nil
}

func (m Model) totalItems() int {
	return len(m.teams) + len(filterCategories)
}

// View implements tea.Model.
func (m Model) View() tea.View {
	// Compute inner dimensions (account for border: 1 char each side).
	innerWidth := m.width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	innerHeight := m.height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	var b strings.Builder

	// Title.
	title := theme.TitleStyle.Render("Teams")
	b.WriteString(title)
	b.WriteString("\n")

	// Show spinner while loading, or empty state message.
	if m.loading {
		b.WriteString("  " + m.spinner.View() + " Loading teams...\n")
	} else if m.loadFailed {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
		b.WriteString("  " + errStyle.Render("Failed to load teams") + "\n")
	} else if len(m.teams) == 0 {
		b.WriteString("  " + theme.SubtitleStyle.Render("No teams found") + "\n")
	}

	// Team list.
	for i, t := range m.teams {
		cursor := "  "
		style := lipgloss.NewStyle()

		if m.section == SectionTeams && i == m.cursor && m.focused {
			cursor = "> "
			style = theme.SelectedStyle
		} else if i == m.selectedTeam {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
		}

		label := truncate(fmt.Sprintf("[%s] %s", t.Key, t.Name), innerWidth-2)
		b.WriteString(cursor + style.Render(label) + "\n")
	}

	// Separator.
	if len(m.teams) > 0 {
		sep := strings.Repeat("─", innerWidth)
		b.WriteString(theme.SubtitleStyle.Render(sep) + "\n")
	}

	// Filter categories title.
	filterTitle := theme.TitleStyle.Render("Filters")
	b.WriteString(filterTitle + "\n")

	for i, f := range filterCategories {
		cursor := "  "
		style := lipgloss.NewStyle()

		if m.section == SectionFilters && i == m.filterCursor && m.focused {
			cursor = "> "
			style = theme.SelectedStyle
		} else if i == m.selectedFilter {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
		}

		b.WriteString(cursor + style.Render(f) + "\n")
	}

	content := b.String()

	// Pad content to fill height.
	lines := strings.Count(content, "\n")
	for lines < innerHeight {
		content += "\n"
		lines++
	}

	// Apply border.
	var borderStyle lipgloss.Style
	if m.focused {
		borderStyle = theme.FocusedBorder
	} else {
		borderStyle = theme.BlurredBorder
	}

	rendered := borderStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(content)

	return tea.NewView(rendered)
}

// truncate truncates a string to the given width.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return s[:maxWidth]
	}
	// Rough truncation; lipgloss.Width accounts for wide chars.
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))  > maxWidth-3 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}
