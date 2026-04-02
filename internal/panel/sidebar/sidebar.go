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
	selectedFilter int
	filters        []string
	filterCounts   map[string]int

	focused        bool
	width          int
	height         int
	loading        bool
	loadFailed     bool
	spinner        spinner.Model
	initialTeamID  string
	initialFilter  string
}

// New creates a new sidebar model.
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	return Model{
		spinner: s,
		filters: []string{
			"My Issues",
			"My Issues + Active",
			"My Unlabeled Issues",
			"All Issues",
			"Active",
		},
	}
}

// isSeparator returns true if the filter entry is a visual separator.
func isSeparator(f string) bool {
	return f == "---"
}

// SetFilterCounts updates the issue counts displayed next to each filter.
func (m *Model) SetFilterCounts(counts map[string]int) {
	m.filterCounts = counts
}

// SetFilters updates the available filters in the sidebar.
func (m *Model) SetFilters(filters []string) {
	m.filterCounts = nil // reset counts when filters change
	m.filters = filters
	// Adjust cursors if needed
	if m.filterCursor >= len(m.filters) {
		m.filterCursor = len(m.filters) - 1
		if m.filterCursor < 0 {
			m.filterCursor = 0
		}
	}
	// Skip separator if cursor landed on one
	m.filterCursor = m.nextSelectableFilter(m.filterCursor, 1)
	if m.selectedFilter >= len(m.filters) {
		m.selectedFilter = 0
	}
}

// SetInitialState sets the initial selected team and filter.
func (m *Model) SetInitialState(teamID, filter string) {
	m.initialTeamID = teamID
	m.initialFilter = filter
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

		var cmds []tea.Cmd
		if len(m.teams) > 0 {
			if m.initialTeamID != "" {
				for i, t := range m.teams {
					if t.ID == m.initialTeamID {
						m.cursor = i
						m.selectedTeam = i
						break
					}
				}
			}
			cmds = append(cmds, func() tea.Msg {
				return appmsg.TeamSelectedMsg{Team: m.teams[m.selectedTeam]}
			})

			if m.initialFilter != "" {
				for i, f := range m.filters {
					if f == m.initialFilter {
						m.filterCursor = i
						m.selectedFilter = i
						cmds = append(cmds, func() tea.Msg {
							return appmsg.FilterSelectedMsg{Filter: f}
						})
						break
					}
				}
			}
		}
		return m, tea.Batch(cmds...)

	case appmsg.FilterCountsMsg:
		m.filterCounts = msg.Counts
		return m, nil

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
	case "j", "down", "ctrl+n":
		m.moveCursor(1)

	case "k", "up", "ctrl+p":
		m.moveCursor(-1)

	case "g":
		// Go to top.
		m.section = SectionTeams
		m.cursor = 0
		m.filterCursor = 0

	case "G":
		// Go to bottom.
		if len(m.filters) > 0 {
			m.section = SectionFilters
			m.filterCursor = len(m.filters) - 1
		} else if len(m.teams) > 0 {
			m.section = SectionTeams
			m.cursor = len(m.teams) - 1
		}

	case "l":
		var cmds []tea.Cmd
		if (m.section == SectionTeams && m.cursor != m.selectedTeam) ||
			(m.section == SectionFilters && m.filterCursor != m.selectedFilter) {
			newM, cmd1 := m.selectItem()
			m = newM.(Model)
			if cmd1 != nil {
				cmds = append(cmds, cmd1)
			}
		}
		cmds = append(cmds, func() tea.Msg { return appmsg.FocusMainPanelMsg{} })
		return m, tea.Batch(cmds...)

	case "esc", "ctrl+[":
		return m, func() tea.Msg { return appmsg.FocusMainPanelMsg{} }

	case "enter":
		return m.selectItem()
	}

	// Auto-select filter if we navigated to it
	if m.section == SectionFilters {
		switch msg.String() {
		case "j", "down", "ctrl+n", "k", "up", "ctrl+p", "g", "G":
			if m.filterCursor != m.selectedFilter {
				return m.selectItem()
			}
		}
	}

	return m, nil
}

// nextSelectableFilter finds the next non-separator filter index in the given direction.
func (m *Model) nextSelectableFilter(from, direction int) int {
	for i := from; i >= 0 && i < len(m.filters); i += direction {
		if !isSeparator(m.filters[i]) {
			return i
		}
	}
	return from
}

func (m *Model) moveCursor(delta int) {
	if m.section == SectionTeams {
		m.cursor += delta
		if m.cursor < 0 {
			m.cursor = 0
		} else if m.cursor >= len(m.teams) {
			// Move to filters section.
			if len(m.filters) > 0 {
				m.section = SectionFilters
				m.filterCursor = m.cursor - len(m.teams)
				if m.filterCursor >= len(m.filters) {
					m.filterCursor = len(m.filters) - 1
				}
				m.filterCursor = m.nextSelectableFilter(m.filterCursor, 1)
				m.cursor = len(m.teams) - 1
			} else {
				m.cursor = len(m.teams) - 1
			}
		}
	} else {
		next := m.filterCursor + delta
		if next < 0 {
			// Move back to teams section.
			if len(m.teams) > 0 {
				m.section = SectionTeams
				m.cursor = len(m.teams) - 1
			} else {
				m.filterCursor = m.nextSelectableFilter(0, 1)
			}
		} else if next >= len(m.filters) {
			m.filterCursor = m.nextSelectableFilter(len(m.filters)-1, -1)
		} else {
			// Skip separators in the direction of movement.
			m.filterCursor = m.nextSelectableFilter(next, delta)
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
	if m.section == SectionFilters && m.filterCursor < len(m.filters) && !isSeparator(m.filters[m.filterCursor]) {
		m.selectedFilter = m.filterCursor
		filter := m.filters[m.filterCursor]
		return m, func() tea.Msg {
			return appmsg.FilterSelectedMsg{Filter: filter}
		}
	}
	return m, nil
}

func (m Model) totalItems() int {
	return len(m.teams) + len(m.filters)
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

	for i, f := range m.filters {
		if isSeparator(f) {
			sep := strings.Repeat("─", innerWidth)
			b.WriteString(theme.SubtitleStyle.Render(sep) + "\n")
			continue
		}

		cursor := "  "
		style := lipgloss.NewStyle()

		if m.section == SectionFilters && i == m.filterCursor && m.focused {
			cursor = "> "
			style = theme.SelectedStyle
		} else if i == m.selectedFilter {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
		}

		filterLabel := f
		if m.filterCounts != nil {
			if count, ok := m.filterCounts[f]; ok {
				countStr := fmt.Sprintf(" (%d)", count)
				if count >= 250 {
					countStr = " (250+)"
				}
				filterLabel = f + countStr
			}
		}
		label := truncate(filterLabel, innerWidth-2)
		b.WriteString(cursor + style.Render(label) + "\n")
	}

	content := b.String()

	// Truncate content to fit within the border's inner height so that
	// the rendered output never exceeds the allocated panel height.
	contentLines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(contentLines) > innerHeight {
		contentLines = contentLines[:innerHeight]
	}
	content = strings.Join(contentLines, "\n")

	// Apply border.
	var borderStyle lipgloss.Style
	if m.focused {
		borderStyle = theme.FocusedBorder
	} else {
		borderStyle = theme.BlurredBorder
	}

	rendered := borderStyle.
		Width(m.width).
		Height(m.height).
		MaxHeight(m.height).
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
