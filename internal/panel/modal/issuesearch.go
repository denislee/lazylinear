package modal

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
)

// IssueSearchModel is a modal for searching issues.
type IssueSearchModel struct {
	list   list.Model
	width  int
	height int
}

// SearchItem wraps a linear.Issue for the search list.
type SearchItem struct {
	Issue linear.Issue
}

func (i SearchItem) Title() string { return i.Issue.Identifier + " " + i.Issue.Title }
func (i SearchItem) Description() string {
	if i.Issue.Assignee != nil {
		return i.Issue.State.Name + " · " + i.Issue.Assignee.Name
	}
	return i.Issue.State.Name + " · Unassigned"
}
func (i SearchItem) FilterValue() string { return i.Issue.Identifier + " " + i.Issue.Title }

// SearchDelegate is a simple delegate for search items.
type SearchDelegate struct{}

func (d SearchDelegate) Height() int                               { return 2 }
func (d SearchDelegate) Spacing() int                              { return 0 }
func (d SearchDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d SearchDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(SearchItem)
	if !ok {
		return
	}

	title := i.Title()
	desc := i.Description()

	if index == m.Index() {
		title = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true).Render("> " + title)
		desc = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Render("  " + desc)
	} else {
		title = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Render("  " + title)
		desc = lipgloss.NewStyle().Foreground(lipgloss.Color("#666")).Render("  " + desc)
	}

	fmt.Fprintf(w, "%s\n%s", ansi.Truncate(title, m.Width(), "..."), ansi.Truncate(desc, m.Width(), "..."))
}

// NewIssueSearch creates a new issue search modal.
func NewIssueSearch(issues []linear.Issue, width, height int) IssueSearchModel {
	items := make([]list.Item, len(issues))
	for i, issue := range issues {
		items[i] = SearchItem{Issue: issue}
	}

	delegate := SearchDelegate{}
	l := list.New(items, delegate, width-10, height-10)
	l.Title = "Search My Issues"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FFF")).
		Padding(0, 1).
		Bold(true)

	// Start in filtering mode for fuzzy search
	l.FilterState()

	return IssueSearchModel{
		list:   l,
		width:  width,
		height: height,
	}
}

// IssueSearchConfirmedMsg is sent when an issue is selected from search.
type IssueSearchConfirmedMsg struct {
	Issue linear.Issue
}

// Update handles messages for the search modal.
func (m IssueSearchModel) Update(msg tea.Msg) (SubModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.list.FilterState() == list.Filtering {
				// If filtering, let the list handle it first.
			} else {
				return m, func() tea.Msg { return appmsg.ModalClosedMsg{} }
			}
		case "enter":
			if i, ok := m.list.SelectedItem().(SearchItem); ok {
				return m, func() tea.Msg {
					return IssueSearchConfirmedMsg{Issue: i.Issue}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the search modal.
func (m IssueSearchModel) View() string {
	return m.list.View()
}
