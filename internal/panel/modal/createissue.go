package modal

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/theme"
)

// IssueCreateConfirmedMsg is sent when the user submits the create issue form.
type IssueCreateConfirmedMsg struct {
	TeamID      string
	Title       string
	Description string
	Priority    int
	StateID     *string
	AssigneeID  *string
	ProjectID   *string
	CycleID     *string
}

// Priority labels indexed by their Linear API values.
var priorities = []string{"None", "Urgent", "High", "Medium", "Low"}

// CreateIssueModel is the issue creation form modal.
type CreateIssueModel struct {
	titleInput     textinput.Model
	descInput      textarea.Model
	priorityCursor int
	statusCursor   int
	assigneeCursor int
	projectCursor  int
	cycleCursor    int
	focusIndex     int // 0=title, 1=desc, 2=priority, 3=status, 4=assignee, 5=project, 6=cycle, 7=submit
	teamID         string
	err            string

	states       []linear.WorkflowState
	assignees    []linear.User
	projects     []linear.Project
	cycles       []linear.Cycle
	listsLoading bool
	currentUser  *linear.User
}

// NewCreateIssue creates a new issue creation form modal.
// The modal opens immediately with empty lists; call SetMetadata once data arrives.
func NewCreateIssue(teamID string, currentUser *linear.User) CreateIssueModel {
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
		titleInput:   ti,
		descInput:    ta,
		teamID:       teamID,
		currentUser:  currentUser,
		listsLoading: true,
	}
}

// SetMetadata populates the assignee, project and cycle lists once data is available.
func (m *CreateIssueModel) SetMetadata(meta *linear.TeamMetadata) {
	m.listsLoading = false
	if meta == nil {
		return
	}

	m.assignees = meta.Members
	m.states = meta.States

	var myProjects []linear.Project
	for _, p := range meta.Projects {
		status := strings.ToLower(p.Status.Name)
		if status == "developing" && m.currentUser != nil && p.Lead != nil && p.Lead.ID == m.currentUser.ID {
			myProjects = append(myProjects, p)
		}
	}
	m.projects = myProjects

	m.cycles = meta.Cycles

	// Set default assignee to current user
	if m.currentUser != nil {
		for i, a := range m.assignees {
			if a.ID == m.currentUser.ID {
				m.assigneeCursor = i + 1 // +1 because 0 is "Unassigned"
				break
			}
		}
	}

	// Set default cycle to current cycle
	now := time.Now()
	for i, c := range m.cycles {
		if now.After(c.StartsAt) && now.Before(c.EndsAt) {
			m.cycleCursor = i + 1 // +1 because 0 is "No Cycle"
			break
		}
	}
}

func (m CreateIssueModel) Update(msg tea.Msg) (SubModal, tea.Cmd) {
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
			m.focusIndex = (m.focusIndex + 1) % 8
			m.updateFocus()
			return m, nil

		case "shift+tab":
			m.focusIndex = (m.focusIndex - 1 + 8) % 8
			m.updateFocus()
			return m, nil

		case "enter":
			if m.focusIndex == 7 && !m.listsLoading {
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
			case 3: // Status
				if m.listsLoading {
					return m, nil
				}
				switch key {
				case "j", "down", "ctrl+n":
					if m.statusCursor < len(m.states) {
						m.statusCursor++
					}
				case "k", "up", "ctrl+p":
					if m.statusCursor > 0 {
						m.statusCursor--
					}
				}
				return m, nil
			case 4: // Assignee
				if m.listsLoading {
					return m, nil
				}
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
			case 5: // Project
				if m.listsLoading {
					return m, nil
				}
				switch key {
				case "j", "down", "ctrl+n":
					if m.projectCursor < len(m.projects) {
						m.projectCursor++
					}
				case "k", "up", "ctrl+p":
					if m.projectCursor > 0 {
						m.projectCursor--
					}
				}
				return m, nil
			case 6: // Cycle
				if m.listsLoading {
					return m, nil
				}
				switch key {
				case "j", "down", "ctrl+n":
					if m.cycleCursor < len(m.cycles) {
						m.cycleCursor++
					}
				case "k", "up", "ctrl+p":
					if m.cycleCursor > 0 {
						m.cycleCursor--
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

func (m CreateIssueModel) submit() (CreateIssueModel, tea.Cmd) {
	title := strings.TrimSpace(m.titleInput.Value())
	if title == "" {
		m.err = "Title is required"
		m.focusIndex = 0
		m.updateFocus()
		return m, nil
	}

	desc := strings.TrimSpace(m.descInput.Value())

	var stateID *string
	if m.statusCursor > 0 {
		id := m.states[m.statusCursor-1].ID
		stateID = &id
	}

	var assigneeID *string
	if m.assigneeCursor > 0 {
		id := m.assignees[m.assigneeCursor-1].ID
		assigneeID = &id
	}

	var projectID *string
	if m.projectCursor > 0 {
		id := m.projects[m.projectCursor-1].ID
		projectID = &id
	}

	var cycleID *string
	if m.cycleCursor > 0 {
		id := m.cycles[m.cycleCursor-1].ID
		cycleID = &id
	}

	return m, func() tea.Msg {
		return IssueCreateConfirmedMsg{
			TeamID:      m.teamID,
			Title:       title,
			Description: desc,
			Priority:    m.priorityCursor,
			StateID:     stateID,
			AssigneeID:  assigneeID,
			ProjectID:   projectID,
			CycleID:     cycleID,
		}
	}
}

func (m CreateIssueModel) View() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Create Issue")
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

	loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666")).Italic(true)

	// Status
	statusLabel := labelStyle.Render("Status:  ")
	if m.focusIndex == 3 {
		statusLabel = focusedLabel.Render("Status:  ")
	}
	b.WriteString(statusLabel + " ")
	if m.listsLoading {
		b.WriteString(loadingStyle.Render("Loading..."))
	} else {
		statusName := "Default"
		if m.statusCursor > 0 {
			statusName = m.states[m.statusCursor-1].Name
		}
		b.WriteString(renderDropdown(statusName, m.focusIndex == 3))
	}
	b.WriteString("\n\n")

	// Assignee
	assLabel := labelStyle.Render("Assignee:")
	if m.focusIndex == 4 {
		assLabel = focusedLabel.Render("Assignee:")
	}
	b.WriteString(assLabel + " ")
	if m.listsLoading {
		b.WriteString(loadingStyle.Render("Loading..."))
	} else {
		assigneeName := "Unassigned"
		if m.assigneeCursor > 0 {
			assigneeName = m.assignees[m.assigneeCursor-1].Name
		}
		b.WriteString(renderDropdown(assigneeName, m.focusIndex == 4))
	}
	b.WriteString("\n\n")

	// Project
	projLabel := labelStyle.Render("Project: ")
	if m.focusIndex == 5 {
		projLabel = focusedLabel.Render("Project: ")
	}
	b.WriteString(projLabel + " ")
	if m.listsLoading {
		b.WriteString(loadingStyle.Render("Loading..."))
	} else {
		projectName := "No Project"
		if m.projectCursor > 0 {
			projectName = m.projects[m.projectCursor-1].Name
		}
		b.WriteString(renderDropdown(projectName, m.focusIndex == 5))
	}
	b.WriteString("\n\n")

	// Cycle
	cycLabel := labelStyle.Render("Cycle:   ")
	if m.focusIndex == 6 {
		cycLabel = focusedLabel.Render("Cycle:   ")
	}
	b.WriteString(cycLabel + " ")
	if m.listsLoading {
		b.WriteString(loadingStyle.Render("Loading..."))
	} else {
		cycleName := "No Cycle"
		if m.cycleCursor > 0 {
			c := m.cycles[m.cycleCursor-1]
			cycleName = fmt.Sprintf("Cycle %d", c.Number)
			if c.Name != "" {
				cycleName = c.Name
			}
		}
		b.WriteString(renderDropdown(cycleName, m.focusIndex == 6))
	}
	b.WriteString("\n\n")

	// Submit button
	submitStyle := lipgloss.NewStyle().Padding(0, 2)
	if m.focusIndex == 7 {
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

func renderDropdown(val string, focused bool) string {
	style := lipgloss.NewStyle()
	if focused {
		style = style.Foreground(lipgloss.Color("#7D56F4")).Bold(true)
		return style.Render(fmt.Sprintf("< %s >", val))
	}
	return style.Render(fmt.Sprintf("[ %s ]", val))
}
