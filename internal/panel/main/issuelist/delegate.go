package issuelist

import (
	"fmt"
	"io"
	"strings"
	"time"

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
	return i.Issue.Identifier + " " + i.Issue.Title + " " + i.Issue.State.Name
}

// IssueDelegate is a custom delegate for rendering issue list items.
type IssueDelegate struct {
	height  int
	spacing int
	Compact bool
}

// NewIssueDelegate creates a new issue delegate.
func NewIssueDelegate() *IssueDelegate {
	return &IssueDelegate{
		height:  2,
		spacing: 1,
		Compact: false,
	}
}

// Height returns the delegate's preferred height.
func (d *IssueDelegate) Height() int {
	if d.Compact {
		return 1
	}
	return d.height
}

// Spacing returns the delegate's spacing.
func (d *IssueDelegate) Spacing() int {
	if d.Compact {
		return 0
	}
	return d.spacing
}

// Update handles item-level updates. We handle custom keys in the parent model
// instead, so this is a no-op.
func (d *IssueDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

// Render renders an issue list item.
func (d *IssueDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
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

	if d.Compact {
		statusStyle := theme.StatusStyle(issue.Issue.State.Type)
		statusBadge := statusStyle.Render("●")
		
		age := formatAge(issue.Issue.CreatedAt)
		ageStr := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666")).
			Render(age)
			
		// Calculate available width for title
		// badge + space + cursor (2) + spaces (2) + age
		availableWidth := textWidth - lipgloss.Width(statusBadge) - lipgloss.Width(ageStr) - 5
		if availableWidth < 10 {
			availableWidth = 10
		}
		
		titleLine := identifier + "  " + title
		titleLine = ansi.Truncate(titleLine, availableWidth, "...")
		
		titleLine = statusBadge + " " + titleLine + "  " + ageStr
		
		if isSelected {
			cursor := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Render("> ")
			titleLine = cursor + titleLine
		} else {
			titleLine = "  " + titleLine
		}
		
		fmt.Fprintf(w, "%s", titleLine) //nolint:errcheck
		return
	}

	// Build the description line: status badge + assignee + project + age.
	statusStyle := theme.StatusStyle(issue.Issue.State.Type)
	statusBadge := statusStyle.Render("● " + issue.Issue.State.Name)

	assignee := "Unassigned"
	if issue.Issue.Assignee != nil {
		assignee = issue.Issue.Assignee.Name
	}
	
	projectStr := ""
	if issue.Issue.Project != nil {
		projectStr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A885FF")).
			Render(issue.Issue.Project.Name) + "  "
	}
	
	age := formatAge(issue.Issue.CreatedAt)
	ageStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666")).
		Render(age)

	assigneeStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render(assignee)

	descLine := statusBadge + "  " + projectStr + assigneeStr + "  " + ageStr
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

// formatAge formats a time.Time into a human-readable age string.
func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
}
