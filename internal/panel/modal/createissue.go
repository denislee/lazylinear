package modal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/theme"
)

// IssueCreateConfirmedMsg is sent when the user submits the create issue form.
type IssueCreateConfirmedMsg struct {
	TeamID      string
	Title       string
	Description string
	Priority    int
}

// Priority labels indexed by their Linear API values.
var priorities = []string{"None", "Urgent", "High", "Medium", "Low"}

// CreateIssueModel is the issue creation form modal.
type CreateIssueModel struct {
	titleInput     textinput.Model
	descInput      textarea.Model
	priorityCursor int
	focusIndex     int // 0=title, 1=desc, 2=priority, 3=submit
	teamID         string
	err            string
}

// NewCreateIssue creates a new issue creation form modal.
func NewCreateIssue(teamID string) CreateIssueModel {
	ti := textinput.New()
	ti.Placeholder = "Issue title"
	ti.CharLimit = 200
	ti.SetWidth(50)
	ti.Focus()

	ta := textarea.New()
	ta.Placeholder = "Description (optional)"
	ta.SetWidth(50)
	ta.SetHeight(4)
	ta.Blur()

	return CreateIssueModel{
		titleInput:     ti,
		descInput:      ta,
		priorityCursor: 0,
		focusIndex:     0,
		teamID:         teamID,
	}
}

// Update handles input for the create issue form.
func (m CreateIssueModel) Update(msg tea.Msg) (CreateIssueModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()

		switch key {
		case "esc":
			return m, func() tea.Msg {
				return appmsg.ModalClosedMsg{}
			}

		case "tab":
			m.focusIndex = (m.focusIndex + 1) % 4
			m.updateFocus()
			return m, nil

		case "shift+tab":
			m.focusIndex = (m.focusIndex - 1 + 4) % 4
			m.updateFocus()
			return m, nil

		case "enter":
			if m.focusIndex == 3 {
				// Submit.
				return m.submit()
			}
			// For other fields, enter is handled below (e.g., newline in textarea).

		default:
			// When focused on priority, handle j/k.
			if m.focusIndex == 2 {
				switch key {
				case "j", "down":
					if m.priorityCursor < len(priorities)-1 {
						m.priorityCursor++
					}
					return m, nil
				case "k", "up":
					if m.priorityCursor > 0 {
						m.priorityCursor--
					}
					return m, nil
				}
				return m, nil
			}
		}
	}

	// Forward messages to the focused input.
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

// updateFocus syncs the focused/blurred state of inputs with focusIndex.
func (m *CreateIssueModel) updateFocus() {
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

// submit validates the form and fires the confirmed message.
func (m CreateIssueModel) submit() (CreateIssueModel, tea.Cmd) {
	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		m.err = "Title is required"
		m.focusIndex = 0
		m.updateFocus()
		return m, nil
	}

	desc := strings.TrimSpace(m.descInput.Value())
	priority := m.priorityCursor // 0=None, 1=Urgent, 2=High, 3=Medium, 4=Low

	return m, func() tea.Msg {
		return IssueCreateConfirmedMsg{
			TeamID:      m.teamID,
			Title:       title,
			Description: desc,
			Priority:    priority,
		}
	}
}

// View renders the create issue form.
func (m CreateIssueModel) View() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Create Issue")
	b.WriteString(title + "\n")
	b.WriteString(theme.SubtitleStyle.Render(strings.Repeat("─", 40)) + "\n\n")

	// Error message.
	if m.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
		b.WriteString(errStyle.Render(m.err) + "\n\n")
	}

	labelStyle := theme.SubtitleStyle
	focusedLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)

	// Title field.
	titleLabel := labelStyle.Render("Title:")
	if m.focusIndex == 0 {
		titleLabel = focusedLabel.Render("Title:")
	}
	b.WriteString(titleLabel + "\n")
	b.WriteString(m.titleInput.View() + "\n\n")

	// Description field.
	descLabel := labelStyle.Render("Description:")
	if m.focusIndex == 1 {
		descLabel = focusedLabel.Render("Description:")
	}
	b.WriteString(descLabel + "\n")
	b.WriteString(m.descInput.View() + "\n\n")

	// Priority selector.
	prioLabel := labelStyle.Render("Priority:")
	if m.focusIndex == 2 {
		prioLabel = focusedLabel.Render("Priority:")
	}
	b.WriteString(prioLabel + "  ")
	for i, p := range priorities {
		style := lipgloss.NewStyle()
		if i == m.priorityCursor {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true)
			b.WriteString(style.Render(fmt.Sprintf("[%s]", p)))
		} else {
			b.WriteString(style.Render(fmt.Sprintf(" %s ", p)))
		}
		if i < len(priorities)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n\n")

	// Submit button.
	submitStyle := lipgloss.NewStyle().
		Padding(0, 2)
	if m.focusIndex == 3 {
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

	b.WriteString(theme.SubtitleStyle.Render("tab/shift+tab: navigate  enter: submit  esc: cancel"))

	return b.String()
}
