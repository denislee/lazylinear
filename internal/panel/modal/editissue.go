package modal

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/theme"
)

// EditIssueModel is the issue edit form modal.
type EditIssueModel struct {
	titleInput     textinput.Model
	descInput      textinput.Model
	priorityCursor int
	assigneeCursor int
	stateCursor    int
	focusIndex     int // 0=title, 1=desc, 2=priority, 3=assignee, 4=state, 5=submit
	issueID        string
	err            string

	assignees []linear.User
	states    []linear.WorkflowState
}

// NewEditIssue creates a new issue edit form modal.
func NewEditIssue(issue linear.Issue, currentUser *linear.User, meta *linear.TeamMetadata) EditIssueModel {
	ti := textinput.New()
	ti.Placeholder = "Issue title"
	ti.SetValue(issue.Title)
	ti.CharLimit = 200
	ti.SetWidth(50)
	ti.Focus()

	ta := textinput.New()
	ta.Placeholder = "Description (optional)"
	ta.SetValue(issue.Description)
	ta.SetWidth(50)
	ta.Blur()

	m := EditIssueModel{
		titleInput:     ti,
		descInput:      ta,
		issueID:        issue.ID, // We use issueID field to store issueID for simplicity
		priorityCursor: issue.Priority,
	}

	if meta != nil {
		m.assignees = meta.Members
		m.states = meta.States
	}

	// Set default assignee to current issue assignee
	if issue.Assignee != nil {
		for i, a := range m.assignees {
			if a.ID == issue.Assignee.ID {
				m.assigneeCursor = i + 1 // +1 because 0 is "Unassigned"
				break
			}
		}
	}

	// Set default state to current issue state
	for i, s := range m.states {
		if s.ID == issue.State.ID {
			m.stateCursor = i
			break
		}
	}

	return m
}

func (m EditIssueModel) Update(msg tea.Msg) (SubModal, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()

		switch key {
		case "esc", "ctrl+[":
			return m, func() tea.Msg {
				return appmsg.ModalClosedMsg{}
			}

		case "tab":
			m.focusIndex = (m.focusIndex + 1) % 6
			m.updateFocus()
			return m, nil

		case "shift+tab":
			m.focusIndex = (m.focusIndex - 1 + 6) % 6
			m.updateFocus()
			return m, nil

		case "enter":
			if m.focusIndex == 5 {
				return m.submit()
			}

		default:
			switch m.focusIndex {
			case 2: // Priority
				switch key {
				case "j", "down", "ctrl+n":
					if m.priorityCursor < len(priorities)-1 {
						m.priorityCursor++
					}
				case "k", "up", "ctrl+p":
					if m.priorityCursor > 0 {
						m.priorityCursor--
					}
				}
				return m, nil
			case 3: // Assignee
				switch key {
				case "j", "down", "ctrl+n":
					if m.assigneeCursor < len(m.assignees) {
						m.assigneeCursor++
					}
				case "k", "up", "ctrl+p":
					if m.assigneeCursor > 0 {
						m.assigneeCursor--
					}
				}
				return m, nil
			case 4: // State
				switch key {
				case "j", "down", "ctrl+n":
					if len(m.states) > 0 && m.stateCursor < len(m.states)-1 {
						m.stateCursor++
					}
				case "k", "up", "ctrl+p":
					if m.stateCursor > 0 {
						m.stateCursor--
					}
				}
				return m, nil
			}
		}
	}

	switch m.focusIndex {
	case 0:
		var cmd tea.Cmd
		m.titleInput, cmd = m.titleInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case 1:
		var cmd tea.Cmd
		m.descInput, cmd = m.descInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *EditIssueModel) updateFocus() {
	m.err = ""
	if m.focusIndex == 0 {
		m.titleInput.Focus()
	} else {
		m.titleInput.Blur()
	}
	if m.focusIndex == 1 {
		m.descInput.Focus()
	} else {
		m.descInput.Blur()
	}
}

func (m EditIssueModel) submit() (EditIssueModel, tea.Cmd) {
	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		m.err = "Title is required"
		m.focusIndex = 0
		m.updateFocus()
		return m, nil
	}

	desc := strings.TrimSpace(m.descInput.Value())

	var assigneeID *string
	if m.assigneeCursor > 0 {
		id := m.assignees[m.assigneeCursor-1].ID
		assigneeID = &id
	}

	var stateID *string
	if len(m.states) > 0 && m.stateCursor >= 0 && m.stateCursor < len(m.states) {
		id := m.states[m.stateCursor].ID
		stateID = &id
	}

	return m, func() tea.Msg {
		priority := m.priorityCursor
		return appmsg.IssueEditConfirmedMsg{
			IssueID:     m.issueID,
			Title:       &title,
			Description: &desc,
			Priority:    &priority,
			AssigneeID:  assigneeID,
			StateID:     stateID,
		}
	}
}

func (m EditIssueModel) View() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Edit Issue")
	b.WriteString(title + "\n")
	b.WriteString(theme.SubtitleStyle.Render(strings.Repeat("─", 40)) + "\n\n")

	if m.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
		b.WriteString(errStyle.Render(m.err) + "\n\n")
	}

	labelStyle := theme.SubtitleStyle
	focusedLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)

	// Title
	titleLabel := labelStyle.Render("Title:")
	if m.focusIndex == 0 {
		titleLabel = focusedLabel.Render("Title:")
	}
	b.WriteString(titleLabel + "\n")
	b.WriteString(m.titleInput.View() + "\n\n")

	// Description
	descLabel := labelStyle.Render("Description:")
	if m.focusIndex == 1 {
		descLabel = focusedLabel.Render("Description:")
	}
	b.WriteString(descLabel + "\n")
	b.WriteString(m.descInput.View() + "\n\n")

	// Priority
	prioLabel := labelStyle.Render("Priority:")
	if m.focusIndex == 2 {
		prioLabel = focusedLabel.Render("Priority:")
	}
	b.WriteString(prioLabel + " ")
	b.WriteString(renderDropdown(priorities[m.priorityCursor], m.focusIndex == 2))
	b.WriteString("\n\n")

	// Assignee
	assigneeName := "Unassigned"
	if m.assigneeCursor > 0 {
		assigneeName = m.assignees[m.assigneeCursor-1].Name
	}
	assLabel := labelStyle.Render("Assignee:")
	if m.focusIndex == 3 {
		assLabel = focusedLabel.Render("Assignee:")
	}
	b.WriteString(assLabel + " ")
	b.WriteString(renderDropdown(assigneeName, m.focusIndex == 3))
	b.WriteString("\n\n")

	// State
	stateName := "Unknown"
	if len(m.states) > 0 && m.stateCursor >= 0 && m.stateCursor < len(m.states) {
		stateName = m.states[m.stateCursor].Name
	}
	stateLabel := labelStyle.Render("State:")
	if m.focusIndex == 4 {
		stateLabel = focusedLabel.Render("State:")
	}
	b.WriteString(stateLabel + " ")
	b.WriteString(renderDropdown(stateName, m.focusIndex == 4))
	b.WriteString("\n\n")

	// Submit button
	submitStyle := lipgloss.NewStyle().Padding(0, 2)
	if m.focusIndex == 5 {
		submitStyle = submitStyle.
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
	} else {
		submitStyle = submitStyle.
			Background(lipgloss.Color("#333")).
			Foreground(lipgloss.Color("#CCC"))
	}
	b.WriteString(submitStyle.Render("Submit") + "\n\n")

	b.WriteString(theme.SubtitleStyle.Render("tab/shift+tab: navigate  j/k: select  enter: submit  esc: cancel"))

	return b.String()
}
