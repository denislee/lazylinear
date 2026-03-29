// Package app message types are defined in internal/msg to avoid import cycles.
// This file re-exports them for convenience within the app package.
package app

import "github.com/denislee/lazylinear/internal/msg"

// Re-export message types so callers within app don't need to change.
type (
	ViewerLoadedMsg         = msg.ViewerLoadedMsg
	TeamsLoadedMsg          = msg.TeamsLoadedMsg
	IssuesLoadedMsg         = msg.IssuesLoadedMsg
	WorkflowStatesLoadedMsg = msg.WorkflowStatesLoadedMsg
	TeamSelectedMsg         = msg.TeamSelectedMsg
	IssueSelectedMsg        = msg.IssueSelectedMsg
	BackToListMsg           = msg.BackToListMsg
	OpenCreateIssueMsg      = msg.OpenCreateIssueMsg
	OpenStatusChangeMsg     = msg.OpenStatusChangeMsg
	ModalClosedMsg          = msg.ModalClosedMsg
	IssueCreatedMsg         = msg.IssueCreatedMsg
	IssueUpdatedMsg         = msg.IssueUpdatedMsg
	FilterSelectedMsg       = msg.FilterSelectedMsg
	RefreshIssuesMsg        = msg.RefreshIssuesMsg
	FocusSidebarMsg         = msg.FocusSidebarMsg
	ErrorMsg                = msg.ErrorMsg
)
