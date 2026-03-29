package issuedetail

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/theme"
)

// priorityLabels maps priority integer values to human-readable labels.
var priorityLabels = map[int]string{
	0: "None",
	1: "Urgent",
	2: "High",
	3: "Medium",
	4: "Low",
}

// Model is the issue detail view. It displays a single issue's
// full information inside a scrollable viewport.
type Model struct {
	viewport viewport.Model
	issue    *linear.Issue
	width    int
	height   int
	focused  bool
}

// New creates a new issue detail model.
func New() Model {
	vp := viewport.New()
	return Model{
		viewport: vp,
	}
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)

	// Re-render content if we have an issue, since width may have changed.
	if m.issue != nil {
		m.viewport.SetContent(m.formatIssue())
	}
}

// SetFocused sets the focus state.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// SetIssue sets the issue to display and formats the content.
func (m *Model) SetIssue(issue linear.Issue) {
	m.issue = &issue
	m.viewport.SetContent(m.formatIssue())
	m.viewport.GotoTop()
}

// Update handles messages for the detail view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "esc", "h", "q":
			return m, func() tea.Msg {
				return appmsg.BackToListMsg{}
			}

		case "s":
			if m.issue != nil {
				issue := *m.issue
				return m, func() tea.Msg {
					return appmsg.OpenStatusChangeMsg{Issue: issue}
				}
			}
			return m, nil
		}
	}

	// Forward all other messages to the viewport for scrolling (j/k/up/down/pgup/pgdn etc.).
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the issue detail.
func (m Model) View() string {
	return m.viewport.View()
}

// formatIssue builds the styled text content for the current issue.
func (m Model) formatIssue() string {
	if m.issue == nil {
		return ""
	}
	issue := m.issue

	contentWidth := m.width
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Padding for content.
	pad := "  "

	var b strings.Builder

	// Header: identifier + title.
	identifier := theme.SelectedStyle.Render(issue.Identifier)
	title := theme.TitleStyle.Render(issue.Title)
	b.WriteString(fmt.Sprintf("%s%s  %s\n", pad, identifier, title))

	// Separator.
	sepWidth := contentWidth - 4
	if sepWidth < 1 {
		sepWidth = 1
	}
	separator := theme.SubtitleStyle.Render(pad + strings.Repeat("─", sepWidth))
	b.WriteString(separator + "\n")

	// Metadata rows.
	labelStyle := theme.SubtitleStyle
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	// Row 1: Status + Priority.
	statusValue := theme.StatusStyle(issue.State.Type).Render(issue.State.Name)
	priorityValue := valueStyle.Render(priorityLabel(issue.Priority))
	b.WriteString(fmt.Sprintf("%s%s  %-16s%s  %s\n",
		pad,
		labelStyle.Render("Status:"),
		statusValue,
		labelStyle.Render("Priority:"),
		priorityValue,
	))

	// Row 2: Assignee + Labels.
	assigneeName := "Unassigned"
	if issue.Assignee != nil {
		assigneeName = issue.Assignee.Name
	}
	labelsStr := "None"
	if len(issue.Labels.Nodes) > 0 {
		names := make([]string, len(issue.Labels.Nodes))
		for i, l := range issue.Labels.Nodes {
			names[i] = l.Name
		}
		labelsStr = strings.Join(names, ", ")
	}
	b.WriteString(fmt.Sprintf("%s%s  %-16s%s  %s\n",
		pad,
		labelStyle.Render("Assignee:"),
		valueStyle.Render(assigneeName),
		labelStyle.Render("Labels:"),
		valueStyle.Render(labelsStr),
	))

	// Row 3: Created + Updated.
	createdStr := issue.CreatedAt.Format("2006-01-02")
	updatedStr := issue.UpdatedAt.Format("2006-01-02")
	b.WriteString(fmt.Sprintf("%s%s  %-16s%s  %s\n",
		pad,
		labelStyle.Render("Created:"),
		valueStyle.Render(createdStr),
		labelStyle.Render("Updated:"),
		valueStyle.Render(updatedStr),
	))

	// Separator.
	b.WriteString(separator + "\n")

	// Description.
	b.WriteString("\n")
	if issue.Description != "" {
		// Wrap description text within the content width with padding.
		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			Width(contentWidth - 4)
		b.WriteString(pad + descStyle.Render(issue.Description) + "\n")
	} else {
		b.WriteString(pad + theme.SubtitleStyle.Render("No description.") + "\n")
	}

	return b.String()
}

// priorityLabel returns the human-readable label for a priority value.
func priorityLabel(p int) string {
	if label, ok := priorityLabels[p]; ok {
		return label
	}
	return "Unknown"
}
