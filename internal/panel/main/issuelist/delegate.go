package issuelist

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/denislee/lazylinear/internal/linear"
	"github.com/denislee/lazylinear/internal/theme"
)

// IssueItem wraps a linear.Issue to implement the list.DefaultItem interface.
type IssueItem struct {
	Issue linear.Issue
}

// Title returns the issue identifier and title.
func (i IssueItem) Title() string {
	return i.Issue.Identifier + "  " + i.Issue.Title
}

// Description returns the status and assignee information.
func (i IssueItem) Description() string {
	parts := []string{i.Issue.State.Name}
	if i.Issue.Assignee != nil {
		parts = append(parts, i.Issue.Assignee.Name)
	} else {
		parts = append(parts, "Unassigned")
	}
	return strings.Join(parts, " · ")
}

// FilterValue returns a string used for filtering.
func (i IssueItem) FilterValue() string {
	return i.Issue.Identifier + " " + i.Issue.Title
}

// IssueDelegate is a custom delegate for rendering issue list items.
type IssueDelegate struct {
	height  int
	spacing int
}

// NewIssueDelegate creates a new issue delegate.
func NewIssueDelegate() IssueDelegate {
	return IssueDelegate{
		height:  2,
		spacing: 1,
	}
}

// Height returns the delegate's preferred height.
func (d IssueDelegate) Height() int {
	return d.height
}

// Spacing returns the delegate's spacing.
func (d IssueDelegate) Spacing() int {
	return d.spacing
}

// Update handles item-level updates. We handle custom keys in the parent model
// instead, so this is a no-op.
func (d IssueDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

// Render renders an issue list item.
func (d IssueDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	issue, ok := item.(IssueItem)
	if !ok {
		return
	}

	if m.Width() <= 0 {
		return
	}

	isSelected := index == m.Index()
	textWidth := m.Width() - 4 // account for padding/cursor
	if textWidth < 1 {
		textWidth = 1
	}

	// Build the title line: identifier + title.
	identifier := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		Render(issue.Issue.Identifier)
	title := issue.Issue.Title

	titleLine := identifier + "  " + title
	titleLine = ansi.Truncate(titleLine, textWidth, "...")

	// Build the description line: status badge + assignee.
	statusStyle := theme.StatusStyle(issue.Issue.State.Type)
	statusBadge := statusStyle.Render("● " + issue.Issue.State.Name)

	assignee := "Unassigned"
	if issue.Issue.Assignee != nil {
		assignee = issue.Issue.Assignee.Name
	}
	assigneeStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render(assignee)

	descLine := statusBadge + "  " + assigneeStr
	descLine = ansi.Truncate(descLine, textWidth, "...")

	// Apply selection styling.
	if isSelected {
		cursor := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Render("> ")
		titleLine = cursor + titleLine
		descLine = "  " + descLine
	} else {
		titleLine = "  " + titleLine
		descLine = "  " + descLine
	}

	fmt.Fprintf(w, "%s\n%s", titleLine, descLine) //nolint:errcheck
}
