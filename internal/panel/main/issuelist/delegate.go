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

const selectedBg = "#5D4399"

// selectedBgPrefix is the ANSI sequence that opens the selection background.
// Inner styled segments end with `\x1b[0m`, which clears the outer bg; we
// splice this prefix in after each reset so the highlight stays continuous
// across segments that set their own foreground.
var selectedBgPrefix = func() string {
	sample := lipgloss.NewStyle().Background(lipgloss.Color(selectedBg)).Render("X")
	if i := strings.Index(sample, "X"); i > 0 {
		return sample[:i]
	}
	return ""
}()

func fillSelectedBg(s string) string {
	// Lipgloss v2 closes styled segments with `\x1b[m` (empty params = full
	// reset), which clears the outer background mid-row. Re-open the bg
	// after each reset so the highlight stays painted across all segments.
	return strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+selectedBgPrefix)
}

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
	height     int
	spacing    int
	Compact    bool
	FilterName string
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

	prio := priorityIndicator(issue.Issue.Priority)
	idColor := "#7D56F4"
	if isSelected {
		// The selection bg is also purple; swap to white so the ID stays legible.
		idColor = "#FFFFFF"
	}
	identifier := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(idColor)).
		Render(issue.Issue.Identifier)
	title := issue.Issue.Title

	if d.Compact {
		statusBadge := theme.StatusStyle(issue.Issue.State.Type).Render("●")
		ageStr := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666")).
			Render(formatAge(issue.Issue.CreatedAt))
		labelsStr := renderLabels(issue.Issue.Labels.Nodes, 1)

		// "My *" filters are scoped to the current user, so the assignee
		// column is redundant — drop it to give more room to the title.
		showAssignee := !strings.HasPrefix(d.FilterName, "My ")
		assigneeStr := ""
		if showAssignee {
			assignee := "—"
			if issue.Issue.Assignee != nil {
				assignee = issue.Issue.Assignee.Name
			}
			assigneeStr = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888")).
				Render(assignee)
		}

		const (
			prioW     = 3
			statusW   = 1
			idW       = 10
			assigneeW = 14
			labelsW   = 8
			ageW      = 5
		)
		reserved := prioW + statusW + idW + labelsW + ageW
		gaps := 5 // gaps between the columns we always render
		if showAssignee {
			reserved += assigneeW
			gaps++
		}
		titleW := textWidth - reserved - gaps
		if titleW < 12 {
			titleW = 12
		}

		cols := []string{
			padOrTruncate(prio, prioW),
			padOrTruncate(statusBadge, statusW),
			padOrTruncate(identifier, idW),
			padOrTruncate(title, titleW),
		}
		if showAssignee {
			cols = append(cols, padOrTruncate(assigneeStr, assigneeW))
		}
		cols = append(cols,
			padOrTruncate(labelsStr, labelsW),
			padOrTruncate(ageStr, ageW),
		)
		line := strings.Join(cols, " ")

		if isSelected {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color(selectedBg)).
				Width(m.Width()).
				Render("  " + fillSelectedBg(line))
		} else {
			line = "  " + line
		}

		fmt.Fprintf(w, "%s", line) //nolint:errcheck
		return
	}

	titleLine := prio + " " + identifier + "  " + title
	titleLine = ansi.Truncate(titleLine, textWidth, "...")

	statusStyle := theme.StatusStyle(issue.Issue.State.Type)
	statusBadge := statusStyle.Render("● " + issue.Issue.State.Name)

	assignee := "Unassigned"
	if issue.Issue.Assignee != nil {
		assignee = issue.Issue.Assignee.Name
	}
	assigneeStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render("@" + assignee)

	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#444")).Render(" · ")
	descParts := []string{statusBadge, assigneeStr}
	if issue.Issue.Project != nil {
		projectStr := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A885FF")).
			Render("▶ " + issue.Issue.Project.Name)
		descParts = append(descParts, projectStr)
	}
	if labelsStr := renderLabels(issue.Issue.Labels.Nodes, 4); labelsStr != "" {
		descParts = append(descParts, labelsStr)
	}
	ageStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666")).
		Render(formatAge(issue.Issue.CreatedAt))
	descParts = append(descParts, ageStr)

	descLine := strings.Join(descParts, sep)
	descLine = ansi.Truncate(descLine, textWidth, "...")

	if isSelected {
		bg := lipgloss.NewStyle().
			Background(lipgloss.Color(selectedBg)).
			Width(m.Width())
		titleLine = bg.Render("  " + fillSelectedBg(titleLine))
		descLine = bg.Render("  " + fillSelectedBg(descLine))
	} else {
		titleLine = "  " + titleLine
		descLine = "  " + descLine
	}

	fmt.Fprintf(w, "%s\n%s", titleLine, descLine) //nolint:errcheck
}

// padOrTruncate pads s with spaces or truncates it (ANSI-aware) so the
// visible width equals width. An empty string becomes pure padding.
func padOrTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w > width {
		return ansi.Truncate(s, width, "…")
	}
	return s + strings.Repeat(" ", width-w)
}

// priorityIndicator returns a styled icon representing an issue's priority.
// Linear priorities: 0=None, 1=Urgent, 2=High, 3=Medium, 4=Low.
func priorityIndicator(p int) string {
	switch p {
	case 1:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#EB5757")).
			Bold(true).
			Render(" ! ")
	case 2:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F2994A")).
			Bold(true).
			Render("▆▆▆")
	case 3:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F2C94C")).
			Render("▆▆▁")
	case 4:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Render("▆▁▁")
	default:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444")).
			Render(" – ")
	}
}

// renderLabels renders up to max labels as colored pills, truncating each
// label name to maxNameLen runes. If more labels exist, it appends a "+N"
// counter in muted color.
func renderLabels(labels []linear.Label, max int) string {
	const maxNameLen = 4
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for i, l := range labels {
		if i >= max {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666")).
				Render(fmt.Sprintf("+%d", len(labels)-i)))
			break
		}
		color := l.Color
		if color == "" {
			color = "#888"
		}
		name := l.Name
		if r := []rune(name); len(r) > maxNameLen {
			name = string(r[:maxNameLen])
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		parts = append(parts, style.Render("●"+name))
	}
	return strings.Join(parts, " ")
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
