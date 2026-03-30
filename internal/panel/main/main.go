package mainpanel

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/panel/main/issuedetail"
	"github.com/denislee/lazylinear/internal/panel/main/issuelist"
	"github.com/denislee/lazylinear/internal/theme"
)

// viewState tracks which sub-view is active.
type viewState int

const (
	listView   viewState = iota
	detailView           // will be used in Phase 4
)

// Model is the main panel container.
// It holds sub-views (issue list, detail) and routes messages to the active one.
type Model struct {
	width       int
	height      int
	focused     bool
	teamName    string
	activeView  viewState
	loading     bool
	issueList   issuelist.Model
	issueDetail issuedetail.Model
	spinner     spinner.Model
}

// New creates a new main panel model.
func New() Model {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))),
	)
	return Model{
		activeView:  listView,
		issueList:   issuelist.New(),
		issueDetail: issuedetail.New(),
		spinner:     s,
	}
}

// SetSize updates the main panel dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Inner dimensions account for the border (1 char each side).
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	innerHeight := height - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	m.issueList.SetSize(innerWidth, innerHeight)
	m.issueDetail.SetSize(innerWidth, innerHeight)
}

// SetFocused sets the focus state of the main panel.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
	m.issueList.SetFocused(focused)
	m.issueDetail.SetFocused(focused)
}

// SetFilterName passes the filter name down to the issue list.
func (m *Model) SetFilterName(name string) {
	m.issueList.SetFilterName(name)
}

// Focused returns whether the main panel is focused.
func (m Model) Focused() bool {
	return m.focused
}

// ToggleCompact toggles the compact mode in the issue list.
func (m *Model) ToggleCompact() {
	m.issueList.ToggleCompact()
}

// SetCompact sets the compact mode in the issue list.
func (m *Model) SetCompact(compact bool) {
	m.issueList.SetCompact(compact)
}

// IsCompact returns whether the issue list is in compact mode.
func (m Model) IsCompact() bool {
	return m.issueList.IsCompact()
}

// IsFiltering returns true if the issue list is in active filtering mode.
// This is used by the app to avoid intercepting keys during filtering.
func (m Model) IsFiltering() bool {
	if m.activeView == listView {
		return m.issueList.IsFiltering()
	}
	return false
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
		m.activeView = listView
		m.loading = true
		m.issueList.SetTeamID(msg.Team.ID)
		return m, m.spinner.Tick

	case appmsg.FilterSelectedMsg:
		m.activeView = listView
		m.loading = true
		return m, m.spinner.Tick

	case appmsg.RefreshIssuesMsg:
		m.loading = true
		return m, m.spinner.Tick

	case appmsg.IssuesLoadedMsg:
		m.loading = false
		// Forward to issue list.
		var cmd tea.Cmd
		m.issueList, cmd = m.issueList.Update(msg)
		return m, cmd

	case appmsg.ErrorMsg:
		// If we were loading, stop the spinner.
		m.loading = false

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case appmsg.IssueSelectedMsg:
		m.activeView = detailView
		m.issueDetail.SetIssue(msg.Issue)
		innerWidth := m.width - 2
		if innerWidth < 1 {
			innerWidth = 1
		}
		innerHeight := m.height - 2
		if innerHeight < 1 {
			innerHeight = 1
		}
		m.issueDetail.SetSize(innerWidth, innerHeight)
		return m, nil

	case appmsg.BackToListMsg:
		m.activeView = listView
		return m, nil

	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}
		return m.routeToActiveView(msg)
	}

	// Forward other messages to the active sub-view.
	return m.routeToActiveView(msg)
}

// routeToActiveView forwards a message to the currently active sub-view.
func (m Model) routeToActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.activeView {
	case listView:
		var cmd tea.Cmd
		m.issueList, cmd = m.issueList.Update(msg)
		return m, cmd
	case detailView:
		var cmd tea.Cmd
		m.issueDetail, cmd = m.issueDetail.Update(msg)
		return m, cmd
	}
	return m, nil
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

	var content string
	if m.teamName == "" {
		content = m.centeredPlaceholder(innerWidth, innerHeight, "Select a team to view issues")
	} else if m.loading {
		content = m.centeredPlaceholder(innerWidth, innerHeight, m.spinner.View()+" Loading issues...")
	} else {
		switch m.activeView {
		case listView:
			content = m.issueList.View()
		case detailView:
			content = m.issueDetail.View()
		default:
			content = m.centeredPlaceholder(innerWidth, innerHeight, "Unknown view")
		}
	}

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
		Render(content)

	return tea.NewView(rendered)
}

// centeredPlaceholder returns text centered both horizontally and vertically.
func (m Model) centeredPlaceholder(width, height int, text string) string {
	textStyle := theme.SubtitleStyle.Width(width).Align(lipgloss.Center)
	line := textStyle.Render(text)

	// Vertical centering.
	topPad := height / 2
	if topPad < 0 {
		topPad = 0
	}

	var b strings.Builder
	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}
	b.WriteString(line)
	return b.String()
}
