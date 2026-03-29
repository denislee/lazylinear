// Package msg defines all custom message types shared across the application.
package msg

import "github.com/denislee/lazylinear/internal/linear"

// ViewerLoadedMsg is sent when the authenticated user has been fetched.
type ViewerLoadedMsg struct {
	User linear.User
}

// TeamsLoadedMsg is sent when teams have been fetched from the API.
type TeamsLoadedMsg struct {
	Teams []linear.Team
}

// IssuesLoadedMsg is sent when issues have been fetched from the API.
type IssuesLoadedMsg struct {
	Issues   []linear.Issue
	PageInfo linear.PageInfo
}

// WorkflowStatesLoadedMsg is sent when workflow states have been fetched.
type WorkflowStatesLoadedMsg struct {
	States []linear.WorkflowState
}

// TeamSelectedMsg is sent when a team is selected in the sidebar.
type TeamSelectedMsg struct {
	Team linear.Team
}

// IssueSelectedMsg is sent when an issue is selected in the issue list.
type IssueSelectedMsg struct {
	Issue linear.Issue
}

// BackToListMsg is sent to navigate back to the issue list.
type BackToListMsg struct{}

// OpenCreateIssueMsg is sent to open the create issue modal.
type OpenCreateIssueMsg struct{}

// OpenStatusChangeMsg is sent to open the status change modal.
type OpenStatusChangeMsg struct {
	Issue linear.Issue
}

// ModalClosedMsg is sent when a modal is closed.
type ModalClosedMsg struct{}

// IssueCreatedMsg is sent when an issue has been created.
type IssueCreatedMsg struct {
	Issue linear.Issue
}

// IssueUpdatedMsg is sent when an issue has been updated.
type IssueUpdatedMsg struct {
	Issue linear.Issue
}

// FilterSelectedMsg is sent when a filter is selected in the sidebar.
type FilterSelectedMsg struct {
	Filter string // filter name: "My Issues", "All Issues", "Active", "Backlog"
}

// RefreshIssuesMsg is sent when the issue list wants to refresh with the current filter.
type RefreshIssuesMsg struct{}

// FocusSidebarMsg is sent when the main panel wants to move focus to the sidebar.
type FocusSidebarMsg struct{}

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct {
	Err error
}
