package modal

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
)

// issueIdentifierPattern matches a full Linear issue identifier such as
// "TECH-12762".
var issueIdentifierPattern = regexp.MustCompile(`^[A-Za-z]+-\d+$`)

const searchTitle = "Search My Issues"

// IssueSearchModel is a modal for searching issues.
type IssueSearchModel struct {
	list   list.Model
	width  int
	height int

	// lookupIdentifier is the identifier-shaped query (e.g. "TECH-12762") we've
	// last requested a direct API lookup for, since the locally-loaded list
	// only contains a recent subset of issues assigned to the current user.
	lookupIdentifier string
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
	l.Title = searchTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	// Disable the default quit bindings so it doesn't conflict with the app
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)

	// Add pagination keys
	l.KeyMap.PrevPage = key.NewBinding(
		key.WithKeys("pgup", "ctrl+b"),
		key.WithHelp("pgup", "prev page"),
	)
	l.KeyMap.NextPage = key.NewBinding(
		key.WithKeys("pgdown", "ctrl+f"),
		key.WithHelp("pgdn", "next page"),
	)

	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FFF")).
		Padding(0, 1).
		Bold(true)

	// Start in filtering mode for fuzzy search
	l.SetFilterText("")
	l.SetFilterState(list.Filtering)

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
	case appmsg.IssueLookupResultMsg:
		if msg.Identifier != m.lookupIdentifier {
			// Stale result for a query the user has since changed; ignore.
			return m, nil
		}
		if msg.Err != nil || msg.Issue == nil {
			m.list.Title = fmt.Sprintf("%s (%s not found)", searchTitle, msg.Identifier)
			return m, nil
		}
		m.list.Title = searchTitle
		return m, m.list.InsertItem(0, SearchItem{Issue: *msg.Issue})

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+[":
			return m, func() tea.Msg { return appmsg.ModalClosedMsg{} }
		case "ctrl+k":
			return m, func() tea.Msg { return appmsg.ModalClosedMsg{} }
		case "ctrl+n":
			m.list.CursorDown()
			return m, nil
		case "ctrl+p":
			m.list.CursorUp()
			return m, nil
		case "ctrl+b":
			m.list.Paginator.PrevPage()
			return m, nil
		case "ctrl+f":
			m.list.Paginator.NextPage()
			return m, nil
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

	cmds := []tea.Cmd{cmd}
	if lookupCmd := m.maybeLookupIdentifier(); lookupCmd != nil {
		cmds = append(cmds, lookupCmd)
	}
	return m, tea.Batch(cmds...)
}

// maybeLookupIdentifier returns a command that fetches an issue directly by
// identifier when the current filter text looks like a full issue code (e.g.
// "TECH-12762") and no locally-loaded issue matches it. The search list only
// contains a recent subset of issues assigned to the current user, so a
// direct lookup is needed to find issues outside that set. Returns nil if no
// lookup is needed.
func (m *IssueSearchModel) maybeLookupIdentifier() tea.Cmd {
	if m.list.FilterState() != list.Filtering {
		return nil
	}

	text := strings.ToUpper(strings.TrimSpace(m.list.FilterValue()))
	if !issueIdentifierPattern.MatchString(text) {
		if m.lookupIdentifier != "" {
			m.lookupIdentifier = ""
			m.list.Title = searchTitle
		}
		return nil
	}

	if text == m.lookupIdentifier {
		return nil
	}
	if len(m.list.VisibleItems()) > 0 {
		return nil
	}

	m.lookupIdentifier = text
	m.list.Title = fmt.Sprintf("%s (looking up %s...)", searchTitle, text)
	return func() tea.Msg {
		return appmsg.LookupIssueByIdentifierMsg{Identifier: text}
	}
}

// View renders the search modal.
func (m IssueSearchModel) View() string {
	return m.list.View()
}
