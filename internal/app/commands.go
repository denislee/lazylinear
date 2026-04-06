package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/denislee/lazylinear/internal/ai"
	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/panel/modal"
)

// fetchViewer returns a command that fetches the authenticated user.
func fetchViewer(ctx *AppContext) tea.Cmd {
	return func() tea.Msg {
		viewer, err := ctx.Client.GetViewer()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch viewer: %w", err)}
		}
		return ViewerLoadedMsg{User: *viewer}
	}
}

// fetchTeams returns a command that fetches the user's teams.
func fetchTeams(ctx *AppContext) tea.Cmd {
	return func() tea.Msg {
		teams, err := ctx.Client.GetTeams()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch teams: %w", err)}
		}
		return TeamsLoadedMsg{Teams: teams}
	}
}

// fetchMyIssues returns a command that fetches all issues assigned to the current user.
func fetchMyIssues(ctx *AppContext) tea.Cmd {
	if ctx.CurrentUser == nil {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("not logged in")}
		}
	}

	filter := map[string]any{
		"assignee": map[string]any{
			"id": map[string]any{"eq": ctx.CurrentUser.ID},
		},
		"state": map[string]any{
			"type": map[string]any{"neq": "completed"},
		},
	}

	return func() tea.Msg {
		conn, err := ctx.Client.GetMyIssues(250, "", filter)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch my issues: %w", err)}
		}
		return MyIssuesLoadedMsg{Issues: conn.Nodes}
	}
}

// fetchIssues returns a command that fetches issues for the given team with an optional status filter.
func fetchIssues(ctx *AppContext, teamID string, filterName string) tea.Cmd {
	filter := buildIssueFilter(filterName, ctx.CurrentUser, ctx.CurrentProjects)
	return func() tea.Msg {
		conn, err := ctx.Client.GetIssues(teamID, 50, "", filter)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch issues: %w", err)}
		}

		return IssuesLoadedMsg{
			Issues:     conn.Nodes,
			PageInfo:   conn.PageInfo,
			FilterName: filterName,
		}
	}
}

// fetchFilterCounts returns a command that fetches issue counts for all non-separator filters.
func fetchFilterCounts(ctx *AppContext, teamID string, filterNames []string) tea.Cmd {
	filterMap := make(map[string]map[string]any)
	for _, name := range filterNames {
		if name == "---" {
			continue
		}
		f := buildIssueFilter(name, ctx.CurrentUser, ctx.CurrentProjects)
		filterMap[name] = f
	}
	return func() tea.Msg {
		counts, err := ctx.Client.GetFilterCounts(teamID, filterMap)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch filter counts: %w", err)}
		}
		return FilterCountsMsg{Counts: counts}
	}
}

// buildIssueFilter converts a sidebar filter name to a Linear GraphQL IssueFilter.
func buildIssueFilter(filterName string, currentUser *linear.User, projects []linear.Project) map[string]any {
	switch filterName {
	case "My Issues":
		if currentUser != nil {
			return map[string]any{
				"assignee": map[string]any{
					"id": map[string]any{"eq": currentUser.ID},
				},
			}
		}
		return nil
	case "My Unlabeled Issues":
		if currentUser != nil {
			return map[string]any{
				"assignee": map[string]any{
					"id": map[string]any{"eq": currentUser.ID},
				},
				"labels": map[string]any{
					"null": true,
				},
			}
		}
		return nil
	case "My Issues + Active":
		if currentUser != nil {
			return map[string]any{
				"and": []map[string]any{
					{
						"assignee": map[string]any{
							"id": map[string]any{"eq": currentUser.ID},
						},
					},
					{
						"state": map[string]any{
							"type": map[string]any{"eq": "started"},
						},
					},
				},
			}
		}
		return nil
	case "Active":
		// Linear state types: "started" covers In Progress, In Review, etc.
		return map[string]any{
			"state": map[string]any{
				"type": map[string]any{"eq": "started"},
			},
		}
	case "All Issues":
		return nil
	}

	// Check for dynamic project filters
	for _, p := range projects {
		projectName := formatProjectNameForFilter(p.Name)
		if filterName == projectName {
			return map[string]any{
				"project": map[string]any{
					"id": map[string]any{"eq": p.ID},
				},
			}
		}
		if filterName == projectName+" + Active" {
			return map[string]any{
				"and": []map[string]any{
					{
						"project": map[string]any{
							"id": map[string]any{"eq": p.ID},
						},
					},
					{
						"state": map[string]any{
							"type": map[string]any{"eq": "started"},
						},
					},
				},
			}
		}
	}

	return nil
}

// formatProjectNameForFilter removes any text between brackets and the brackets themselves.
func formatProjectNameForFilter(name string) string {
	var result []rune
	inBracket := false
	for _, r := range name {
		if r == '[' {
			inBracket = true
			continue
		}
		if r == ']' {
			inBracket = false
			continue
		}
		if !inBracket {
			result = append(result, r)
		}
	}
	return strings.TrimSpace(string(result))
}

// fetchWorkflowStates returns a command that fetches workflow states for a team.
func fetchWorkflowStates(ctx *AppContext, teamID string) tea.Cmd {
	return func() tea.Msg {
		states, err := ctx.Client.GetWorkflowStates(teamID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch workflow states: %w", err)}
		}
		return WorkflowStatesLoadedMsg{States: states}
	}
}

// fetchTeamMetadata returns a command that fetches team metadata (members, projects, cycles).
func fetchTeamMetadata(ctx *AppContext, teamID string) tea.Cmd {
	return func() tea.Msg {
		meta, err := ctx.Client.GetTeamMetadata(teamID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch team metadata: %w", err)}
		}
		return TeamMetadataLoadedMsg{Metadata: meta}
	}
}

// updateIssueStatus returns a command that updates an issue's workflow state.
func updateIssueStatus(ctx *AppContext, issueID, stateID string) tea.Cmd {
	return func() tea.Msg {
		updated, err := ctx.Client.UpdateIssue(issueID, linear.IssueUpdateInput{
			StateID: &stateID,
		})
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("update issue status: %w", err)}
		}
		return IssueUpdatedMsg{Issue: *updated}
	}
}

// createIssue returns a command that creates a new issue.
func createIssue(ctx *AppContext, confirmed modal.IssueCreateConfirmedMsg) tea.Cmd {
	return func() tea.Msg {
		input := linear.IssueCreateInput{
			TeamID: confirmed.TeamID,
			Title:  confirmed.Title,
		}
		if confirmed.Description != "" {
			desc := confirmed.Description
			input.Description = &desc
		}
		if confirmed.Priority > 0 {
			prio := confirmed.Priority
			input.Priority = &prio
		}
		if confirmed.AssigneeID != nil {
			input.AssigneeID = confirmed.AssigneeID
		}
		if confirmed.ProjectID != nil {
			input.ProjectID = confirmed.ProjectID
		}
		if confirmed.StateID != nil {
			input.StateID = confirmed.StateID
		}
		if confirmed.CycleID != nil {
			input.CycleID = confirmed.CycleID
		}

		issue, err := ctx.Client.CreateIssue(input)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("create issue: %w", err)}
		}
		return IssueCreatedMsg{Issue: *issue}
	}
}

// editIssue returns a command that edits an existing issue.
func editIssue(ctx *AppContext, confirmed IssueEditConfirmedMsg) tea.Cmd {
	return func() tea.Msg {
		input := linear.IssueUpdateInput{
			Title:       confirmed.Title,
			Description: confirmed.Description,
		}
		if confirmed.Priority != nil {
			input.Priority = confirmed.Priority
		}
		if confirmed.AssigneeID != nil {
			input.AssigneeID = confirmed.AssigneeID
		}
		if confirmed.StateID != nil {
			input.StateID = confirmed.StateID
		}

		issue, err := ctx.Client.UpdateIssue(confirmed.IssueID, input)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("edit issue: %w", err)}
		}
		return IssueUpdatedMsg{Issue: *issue}
	}
}

var allowedLabels = []string{
	"Bug",
	"New Feature",
	"Feature Improvement",
	"Investigation",
	"System Improvement",
	"Housekeeping",
	"Documentation",
}

// autoTagIssues returns a command that auto-tags issues using Gemini CLI.
func autoTagIssues(ctx *AppContext, issues []linear.Issue) tea.Cmd {
	if len(issues) == 0 {
		return nil
	}

	return func() tea.Msg {
		if ctx.CurrentTeam == nil {
			return ErrorMsg{Err: fmt.Errorf("no current team")}
		}

		meta, err := ctx.Client.GetTeamMetadata(ctx.CurrentTeam.ID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch team metadata: %w", err)}
		}

		labelMap := make(map[string]string)
		for _, l := range meta.Labels {
			labelMap[l.Name] = l.ID
		}

		existingAllowed := []string{}
		for _, name := range allowedLabels {
			if _, ok := labelMap[name]; ok {
				existingAllowed = append(existingAllowed, name)
			}
		}

		if len(existingAllowed) == 0 {
			return ErrorMsg{Err: fmt.Errorf("none of the allowed labels exist in this team")}
		}

		return appmsg.AutoLabelStartMsg{
			Issues:   issues,
			LabelMap: labelMap,
			Allowed:  existingAllowed,
		}
	}
}

func processNextIssue(ctx *AppContext, issue linear.Issue, curr, total int, allowed []string, labelMap map[string]string) tea.Cmd {
	return func() tea.Msg {
		aiClient := ai.NewGeminiClient()
		category, err := aiClient.CategorizeIssue(issue.Identifier, issue.Title, issue.Description, allowed)
		if err != nil {
			return appmsg.AutoLabelProgressMsg{
				Message: fmt.Sprintf("[%d/%d] Skipped %s (error: %v)", curr, total, issue.Identifier, err),
			}
		}

		var labelID string
		var labelName string
		for _, l := range allowed {
			if strings.Contains(strings.ToLower(category), strings.ToLower(l)) {
				labelID = labelMap[l]
				labelName = l
				break
			}
		}

		if labelID != "" {
			if err := ctx.Client.UpdateIssueLabels(issue.ID, []string{labelID}); err == nil {
				return appmsg.AutoLabelProgressMsg{
					Message: fmt.Sprintf("[%d/%d] Request: %s -> Response: %s", curr, total, issue.Identifier, labelName),
				}
			}
		}

		return appmsg.AutoLabelProgressMsg{
			Message: fmt.Sprintf("[%d/%d] Skipped %s (unknown category: %s)", curr, total, issue.Identifier, category),
		}
	}
}

// openBrowser opens the specified URL in the default web browser.
func openBrowser(url string) tea.Cmd {
	if url == "" {
		return nil
	}
	return func() tea.Msg {
		var err error
		switch runtime.GOOS {
		case "linux":
			err = exec.Command("xdg-open", url).Start()
		case "windows":
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		case "darwin":
			err = exec.Command("open", url).Start()
		default:
			err = fmt.Errorf("unsupported platform: %s", runtime.GOOS)
		}
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("open browser: %w", err)}
		}
		return nil
	}
}