// Package modal provides overlay modals for the application.
package modal

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/linear"
	"github.com/denislee/lazylinear/internal/theme"
)

// ModalType identifies which modal is currently active.
type ModalType int

const (
	// None means no modal is active.
	None ModalType = iota
	// StatusChange is the status picker modal.
	StatusChange
	// CreateIssue is the issue creation form modal.
	CreateIssue
	// EditIssue is the issue edit form modal.
	EditIssue
)

// Model is the modal manager. It holds the active modal state and routes
// input and rendering to the appropriate sub-modal.
type Model struct {
	modalType    ModalType
	statusChange StatusChangeModel
	createIssue  CreateIssueModel
	editIssue    EditIssueModel
	width        int
	height       int
}

// New creates a new modal manager with no active modal.
func New() Model {
	return Model{
		modalType: None,
	}
}

// Active returns true if a modal is currently open.
func (m Model) Active() bool {
	return m.modalType != None
}

// SetSize stores the terminal dimensions for centering the overlay.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// OpenStatusChange opens the status change modal for the given issue and states.
func (m *Model) OpenStatusChange(issue linear.Issue, states []linear.WorkflowState) {
	m.statusChange = NewStatusChange(issue, states)
	m.modalType = StatusChange
}

// OpenCreateIssue opens the create issue modal for the given team.
func (m *Model) OpenCreateIssue(teamID string, currentUser *linear.User, meta *linear.TeamMetadata) {
	m.createIssue = NewCreateIssue(teamID, currentUser, meta)
	m.modalType = CreateIssue
}

// OpenEditIssue opens the edit issue modal for the given issue.
func (m *Model) OpenEditIssue(issue linear.Issue, currentUser *linear.User, meta *linear.TeamMetadata) {
	m.editIssue = NewEditIssue(issue, currentUser, meta)
	m.modalType = EditIssue
}

// Close closes the active modal.
func (m *Model) Close() {
	m.modalType = None
}

// Update routes messages to the active modal.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m.modalType {
	case StatusChange:
		var cmd tea.Cmd
		m.statusChange, cmd = m.statusChange.Update(msg)
		return m, cmd
	case CreateIssue:
		var cmd tea.Cmd
		m.createIssue, cmd = m.createIssue.Update(msg)
		return m, cmd
	case EditIssue:
		var cmd tea.Cmd
		m.editIssue, cmd = m.editIssue.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the active modal wrapped in the modal style and centered on screen.
func (m Model) View() string {
	if m.modalType == None {
		return ""
	}

	var content string
	switch m.modalType {
	case StatusChange:
		content = m.statusChange.View()
	case CreateIssue:
		content = m.createIssue.View()
	case EditIssue:
		content = m.editIssue.View()
	}

	// Wrap in modal style.
	modalContent := theme.ModalStyle.Render(content)

	// Center on screen.
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalContent)
}
