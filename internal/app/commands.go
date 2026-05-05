package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

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

// fetchLeadingProjects returns a command that fetches projects led by the user with status "Developing".
func fetchLeadingProjects(ctx *AppContext) tea.Cmd {
	if ctx.CurrentUser == nil {
		return nil
	}
	return func() tea.Msg {
		projects, err := ctx.Client.GetLeadingProjects(ctx.CurrentUser.ID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch leading projects: %w", err)}
		}
		return appmsg.LeadingProjectsLoadedMsg{Projects: projects}
	}
}

// copyProjectIssues returns a command that fetches project issues from the last cycle and copies them to the clipboard.
func copyProjectIssues(ctx *AppContext, project linear.Project) tea.Cmd {
	return func() tea.Msg {
		titles, err := ctx.Client.GetProjectIssuesFromLastCycle(project.ID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("copy project issues: %w", err)}
		}

		if len(titles) == 0 {
			return ErrorMsg{Err: fmt.Errorf("no completed issues found in the last cycle of %s", project.Name)}
		}

		for i, title := range titles {
			titles[i] = "- " + title
		}

		text := strings.Join(titles, "\n")
		if err := clipboard.WriteAll(text); err != nil {
			return ErrorMsg{Err: fmt.Errorf("write to clipboard: %w", err)}
		}

		return appmsg.ProjectIssuesCopiedMsg{
			ProjectName: project.Name,
			Count:       len(titles),
		}
	}
}

// copyCycleIssues returns a command that copies cycle issues to the clipboard.
func copyCycleIssues(cycle linear.Cycle, issues []linear.Issue) tea.Cmd {
	return func() tea.Msg {
		if len(issues) == 0 {
			return ErrorMsg{Err: fmt.Errorf("no issues found in Cycle %d", cycle.Number)}
		}

		titles := make([]string, len(issues))
		for i, issue := range issues {
			titles[i] = "- " + issue.Title
		}

		text := strings.Join(titles, "\n")
		if err := clipboard.WriteAll(text); err != nil {
			return ErrorMsg{Err: fmt.Errorf("write to clipboard: %w", err)}
		}

		return appmsg.CycleIssuesCopiedMsg{
			CycleNumber: cycle.Number,
			CycleName:   cycle.Name,
			Count:       len(issues),
		}
	}
}

// fetchProjectCycles returns a command that fetches project issues grouped by cycles.
func fetchProjectCycles(ctx *AppContext, project linear.Project) tea.Cmd {
	return func() tea.Msg {
		cycles, err := ctx.Client.GetProjectIssuesByCycles(project.ID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch project cycles: %w", err)}
		}
		return appmsg.ProjectCyclesLoadedMsg{
			ProjectName: project.Name,
			Cycles:      cycles,
		}
	}
}

// fetchMyIssues returns a command that fetches issues for the search modal.
// We now fetch up to 250 recent issues for the current team assigned to the current user,
// including completed and archived ones.
func fetchMyIssues(ctx *AppContext) tea.Cmd {
	if ctx.CurrentTeam == nil {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("no team selected")}
		}
	}
	if ctx.CurrentUser == nil {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("not logged in")}
		}
	}

	filter := map[string]any{
		"assignee": map[string]any{
			"id": map[string]any{"eq": ctx.CurrentUser.ID},
		},
	}

	return func() tea.Msg {
		// Pass an assignee filter and includeArchived=true to get all issues for the user in the team
		conn, err := ctx.Client.GetIssues(ctx.CurrentTeam.ID, 250, "", filter, true)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch search issues: %w", err)}
		}
		return MyIssuesLoadedMsg{Issues: conn.Nodes}
	}
}

// fetchIssues returns a command that fetches issues for the given team with an optional status filter.
func fetchIssues(ctx *AppContext, teamID string, filterName string) tea.Cmd {
	filter := buildIssueFilter(filterName, ctx.CurrentUser, ctx.CurrentProjects)
	return func() tea.Msg {
		conn, err := ctx.Client.GetIssues(teamID, 50, "", filter, false)
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
					"length": map[string]any{"eq": 0},
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
		if confirmed.ProjectID != nil {
			input.ProjectID = confirmed.ProjectID
		}

		issue, err := ctx.Client.UpdateIssue(confirmed.IssueID, input)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("edit issue: %w", err)}
		}
		return IssueUpdatedMsg{Issue: *issue}
	}
}

// autoTagIssues returns a command that auto-tags issues using Gemini CLI.
// It mirrors the logic of linear_labeler.py:
//   1. Fetch every label in the workspace (paginated).
//   2. Build team-specific + org-wide label name->id maps (skipping group labels).
//   3. Use all discovered label names as allowed categories.
//   4. Ask Gemini once for all issues in a single batch call.
//   5. Parse "ID: Category" lines, preferring the longest matching label name.
//   6. Apply suggestions, creating team labels on demand if missing.
func autoTagIssues(ctx *AppContext, issues []linear.Issue) tea.Cmd {
	if len(issues) == 0 {
		return nil
	}

	return func() tea.Msg {
		if ctx.CurrentTeam == nil {
			return ErrorMsg{Err: fmt.Errorf("no current team")}
		}
		teamID := ctx.CurrentTeam.ID

		all, err := ctx.Client.GetAllIssueLabels()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch issue labels: %w", err)}
		}

		teamLabels := make(map[string]string)
		orgLabels := make(map[string]string)
		nameSet := make(map[string]struct{})
		for _, l := range all {
			if l.IsGroup {
				continue
			}
			if l.TeamID == "" {
				orgLabels[l.Name] = l.ID
			} else if l.TeamID == teamID {
				teamLabels[l.Name] = l.ID
			}
			// Allowed label universe includes every non-group label in the
			// workspace — matching the default behavior of linear_labeler.py
			// when --label-group is not set.
			nameSet[l.Name] = struct{}{}
		}
		allowed := make([]string, 0, len(nameSet))
		for name := range nameSet {
			allowed = append(allowed, name)
		}
		sort.Strings(allowed)
		if len(allowed) == 0 {
			return ErrorMsg{Err: fmt.Errorf("no labels found in workspace")}
		}

		inputs := make([]ai.IssueInput, 0, len(issues))
		for _, i := range issues {
			inputs = append(inputs, ai.IssueInput{
				Identifier:  i.Identifier,
				Title:       i.Title,
				Description: i.Description,
			})
		}

		suggestions, err := ai.NewGeminiClient().CategorizeIssues(inputs, allowed)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("gemini categorize: %w", err)}
		}

		return appmsg.AutoLabelStartMsg{
			Issues:      issues,
			Suggestions: suggestions,
			TeamLabels:  teamLabels,
			OrgLabels:   orgLabels,
			TeamID:      teamID,
		}
	}
}

// applyIssueLabel resolves the label for a single issue (creating one if
// necessary) and applies it, emitting a progress message.
func applyIssueLabel(ctx *AppContext, issue linear.Issue, suggested string, curr, total int, teamLabels, orgLabels map[string]string, teamID string) tea.Cmd {
	return func() tea.Msg {
		if suggested == "" {
			return appmsg.AutoLabelProgressMsg{
				Message: fmt.Sprintf("[%d/%d] %s: NOT CATEGORIZED", curr, total, issue.Identifier),
			}
		}

		labelID := teamLabels[suggested]
		if labelID == "" {
			labelID = orgLabels[suggested]
		}
		if labelID == "" {
			id, err := ctx.Client.CreateLabel(suggested, teamID)
			if err != nil {
				return appmsg.AutoLabelProgressMsg{
					Message: fmt.Sprintf("[%d/%d] %s: create label %q failed: %v", curr, total, issue.Identifier, suggested, err),
				}
			}
			teamLabels[suggested] = id
			labelID = id
		}

		if err := ctx.Client.UpdateIssueLabels(issue.ID, []string{labelID}); err != nil {
			return appmsg.AutoLabelProgressMsg{
				Message: fmt.Sprintf("[%d/%d] %s: update failed: %v", curr, total, issue.Identifier, err),
			}
		}
		return appmsg.AutoLabelProgressMsg{
			Message: fmt.Sprintf("[%d/%d] %s -> %s", curr, total, issue.Identifier, suggested),
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