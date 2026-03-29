package modal

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/theme"
)

// StatusChangeConfirmedMsg is sent when the user confirms a status change.
type StatusChangeConfirmedMsg struct {
	IssueID    string
	NewStateID string
}

// StatusChangeModel is the status picker modal.
type StatusChangeModel struct {
	states         []linear.WorkflowState
	currentStateID string
	cursor         int
	issueID        string
}

// NewStatusChange creates a new status change modal for the given issue and workflow states.
func NewStatusChange(issue linear.Issue, states []linear.WorkflowState) StatusChangeModel {
	// Find the index of the current state so the cursor starts there.
	cursor := 0
	for i, s := range states {
		if s.ID == issue.State.ID {
			cursor = i
			break
		}
	}

	return StatusChangeModel{
		states:         states,
		currentStateID: issue.State.ID,
		cursor:         cursor,
		issueID:        issue.ID,
	}
}

// Update handles input for the status change modal.
func (m StatusChangeModel) Update(msg tea.Msg) (StatusChangeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.states)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.states) > 0 {
				selected := m.states[m.cursor]
				return m, func() tea.Msg {
					return StatusChangeConfirmedMsg{
						IssueID:    m.issueID,
						NewStateID: selected.ID,
					}
				}
			}
		case "esc":
			return m, func() tea.Msg {
				return appmsg.ModalClosedMsg{}
			}
		}
	}
	return m, nil
}

// View renders the status change modal content.
func (m StatusChangeModel) View() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Change Status")
	b.WriteString(title + "\n")
	b.WriteString(theme.SubtitleStyle.Render(strings.Repeat("─", 40)) + "\n\n")

	for i, state := range m.states {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		stateStyle := theme.StatusStyle(state.Type)
		name := stateStyle.Render(state.Name)

		// Mark the current state.
		current := ""
		if state.ID == m.currentStateID {
			current = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888")).
				Render(" (current)")
		}

		// Highlight the cursor line.
		if i == m.cursor {
			cursor = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true).
				Render("> ")
		}

		b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, name, current))
	}

	b.WriteString("\n" + theme.SubtitleStyle.Render("j/k: navigate  enter: confirm  esc: cancel"))

	return b.String()
}
