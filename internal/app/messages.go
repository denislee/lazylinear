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
	TeamMetadataLoadedMsg   = msg.TeamMetadataLoadedMsg
	TeamSelectedMsg         = msg.TeamSelectedMsg
	IssueSelectedMsg        = msg.IssueSelectedMsg
	OpenIssueInBrowserMsg   = msg.OpenIssueInBrowserMsg
	BackToListMsg           = msg.BackToListMsg
	OpenCreateIssueMsg      = msg.OpenCreateIssueMsg
	OpenEditIssueMsg        = msg.OpenEditIssueMsg
	OpenStatusChangeMsg     = msg.OpenStatusChangeMsg
	OpenIssueSearchMsg      = msg.OpenIssueSearchMsg
	MyIssuesLoadedMsg       = msg.MyIssuesLoadedMsg
	ModalClosedMsg          = msg.ModalClosedMsg
	IssueEditConfirmedMsg   = msg.IssueEditConfirmedMsg
	IssueCreatedMsg         = msg.IssueCreatedMsg
	IssueUpdatedMsg         = msg.IssueUpdatedMsg
	FilterSelectedMsg       = msg.FilterSelectedMsg
	RefreshIssuesMsg        = msg.RefreshIssuesMsg
	AutoTagIssuesMsg        = msg.AutoTagIssuesMsg
	FilterCountsMsg         = msg.FilterCountsMsg
	FocusSidebarMsg         = msg.FocusSidebarMsg
	FocusMainPanelMsg       = msg.FocusMainPanelMsg
	ErrorMsg                = msg.ErrorMsg
)
