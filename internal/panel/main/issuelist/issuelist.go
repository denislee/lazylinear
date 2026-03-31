package issuelist

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
)

// Model wraps a bubbles list.Model to display issues.
type Model struct {
	list     list.Model
	teamID   string
	pageInfo linear.PageInfo
	focused  bool
	compact  bool
}

// New creates a new issue list model.
func New() Model {
	delegate := NewIssueDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Issues"
	l.SetShowHelp(false)
	l.SetStatusBarItemName("issue", "issues")
	l.SetShowStatusBar(true)

	// Disable the default quit key binding so it doesn't conflict
	// with the app-level quit.
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)

	// Unbind h/l from pagination so we can use them for navigation.
	l.KeyMap.PrevPage = key.NewBinding(
		key.WithKeys("pgup", "b", "u", "ctrl+b"),
		key.WithHelp("pgup", "prev page"),
	)
	l.KeyMap.NextPage = key.NewBinding(
		key.WithKeys("pgdown", "f", "d", "ctrl+f"),
		key.WithHelp("pgdn", "next page"),
	)

	l.KeyMap.CursorUp = key.NewBinding(
		key.WithKeys("up", "k", "ctrl+p"),
		key.WithHelp("↑/k", "up"),
	)
	l.KeyMap.CursorDown = key.NewBinding(
		key.WithKeys("down", "j", "ctrl+n"),
		key.WithHelp("↓/j", "down"),
	)

	l.KeyMap.CancelWhileFiltering = key.NewBinding(
		key.WithKeys("esc", "ctrl+["),
		key.WithHelp("esc", "cancel filter"),
	)
	l.KeyMap.ClearFilter = key.NewBinding(
		key.WithKeys("esc", "ctrl+["),
		key.WithHelp("esc", "clear filter"),
	)

	return Model{
		list: l,
	}
}

// SetSize updates the list dimensions.
func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// SetFocused sets the focus state.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// ToggleCompact toggles the compact mode of the list delegate.
func (m *Model) ToggleCompact() {
	m.SetCompact(!m.compact)
}

// SetCompact sets the compact mode of the list delegate.
func (m *Model) SetCompact(compact bool) {
	m.compact = compact
	delegate := NewIssueDelegate()
	delegate.Compact = m.compact
	m.list.SetDelegate(delegate)
}

// IsCompact returns whether the list is in compact mode.
func (m Model) IsCompact() bool {
	return m.compact
}

// Focused returns whether the list is focused.
func (m Model) Focused() bool {
	return m.focused
}

// IsFiltering returns true if the list is in active filtering mode.
func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

// SetTeamID sets the team whose issues are displayed, and clears the list.
func (m *Model) SetTeamID(teamID string) {
	m.teamID = teamID
	m.pageInfo = linear.PageInfo{}
	m.list.SetItems(nil)
}

// SetFilterName updates the displayed title of the list.
func (m *Model) SetFilterName(name string) {
	m.list.Title = "Issues - " + name
}

// TeamID returns the current team ID.
func (m Model) TeamID() string {
	return m.teamID
}

// Update handles messages and returns the updated model and any command.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case appmsg.IssuesLoadedMsg:
		items := make([]list.Item, len(msg.Issues))
		for i, issue := range msg.Issues {
			items[i] = IssueItem{Issue: issue}
		}
		cmd := m.list.SetItems(items)
		m.pageInfo = msg.PageInfo
		return m, cmd

	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}

		// If the list is in filtering mode, let it handle all keys.
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		// Handle our custom keys before passing to the list.
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(IssueItem); ok {
				return m, func() tea.Msg {
					return appmsg.OpenIssueInBrowserMsg{Issue: item.Issue}
				}
			}
			return m, nil

		case "l":
			if item, ok := m.list.SelectedItem().(IssueItem); ok {
				return m, func() tea.Msg {
					return appmsg.IssueSelectedMsg{Issue: item.Issue}
				}
			}
			return m, nil

		case "e":
			if item, ok := m.list.SelectedItem().(IssueItem); ok {
				return m, func() tea.Msg {
					return appmsg.OpenEditIssueMsg{Issue: item.Issue}
				}
			}
			return m, nil

		case "s":
			if item, ok := m.list.SelectedItem().(IssueItem); ok {
				return m, func() tea.Msg {
					return appmsg.OpenStatusChangeMsg{Issue: item.Issue}
				}
			}
			return m, nil

		case "T":
			// Extract all issues from the list.
			var issues []linear.Issue
			for _, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok {
					issues = append(issues, issueItem.Issue)
				}
			}
			return m, func() tea.Msg {
				return appmsg.AutoTagIssuesMsg{Issues: issues}
			}

		case "r":
			return m, func() tea.Msg {
				return appmsg.RefreshIssuesMsg{}
			}

		case "h", "esc", "ctrl+[":
			return m, func() tea.Msg {
				return appmsg.FocusSidebarMsg{}
			}
		}
	}

	// Forward all other messages to the inner list.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the issue list.
func (m Model) View() string {
	return m.list.View()
}
