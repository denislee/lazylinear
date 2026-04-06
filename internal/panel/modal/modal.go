// Package modal provides overlay modals for the application.
package modal

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/linear"
	"github.com/denislee/lazylinear/internal/theme"
)

// SubModal defines the interface for sub-modals managed by Model.
type SubModal interface {
	Update(msg tea.Msg) (SubModal, tea.Cmd)
	View() string
}

// Model is the modal manager. It holds the active modal state and routes
// input and rendering to the appropriate sub-modal.
type Model struct {
	activeModal SubModal
	width       int
	height      int
}

// New creates a new modal manager with no active modal.
func New() Model {
	return Model{}
}

// Active returns true if a modal is currently open.
func (m Model) Active() bool {
	return m.activeModal != nil
}

// SetSize stores the terminal dimensions for centering the overlay.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// OpenStatusChange opens the status change modal for the given issue and states.
func (m *Model) OpenStatusChange(issue linear.Issue, states []linear.WorkflowState) {
	m.activeModal = NewStatusChange(issue, states)
}

// OpenCreateIssue opens the create issue modal for the given team.
// Lists are initially empty; call SetCreateIssueMetadata to populate them.
func (m *Model) OpenCreateIssue(teamID string, currentUser *linear.User) {
	m.activeModal = NewCreateIssue(teamID, currentUser)
}

// SetCreateIssueMetadata populates the assignee/project/cycle lists on the active create issue modal.
func (m *Model) SetCreateIssueMetadata(meta *linear.TeamMetadata) {
	if ci, ok := m.activeModal.(CreateIssueModel); ok {
		ci.SetMetadata(meta)
		m.activeModal = ci
	}
}

// OpenEditIssue opens the edit issue modal for the given issue.
func (m *Model) OpenEditIssue(issue linear.Issue, currentUser *linear.User, meta *linear.TeamMetadata) {
	m.activeModal = NewEditIssue(issue, currentUser, meta)
}

// OpenIssueSearch opens the issue search modal.
func (m *Model) OpenIssueSearch(issues []linear.Issue) {
	m.activeModal = NewIssueSearch(issues, m.width, m.height)
}

// Close closes the active modal.
func (m *Model) Close() {
	m.activeModal = nil
}

// Update routes messages to the active modal.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.activeModal != nil {
		var cmd tea.Cmd
		m.activeModal, cmd = m.activeModal.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the active modal wrapped in the modal style and centered on screen.
func (m Model) View() string {
	if m.activeModal == nil {
		return ""
	}

	content := m.activeModal.View()

	// Wrap in modal style.
	modalContent := theme.ModalStyle.Render(content)

	// Center on screen.
	res := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalContent)
	return res
}
