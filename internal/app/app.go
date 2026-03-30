package app

import (
	"fmt"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/config"
	"github.com/denislee/lazylinear/internal/linear"
	mainpanel "github.com/denislee/lazylinear/internal/panel/main"
	"github.com/denislee/lazylinear/internal/panel/modal"
	"github.com/denislee/lazylinear/internal/panel/sidebar"
	"github.com/denislee/lazylinear/internal/panel/statusbar"
)

// PanelID identifies which panel has focus.
type PanelID int

const (
	PanelSidebar PanelID = iota
	PanelMain
)

// App is the root Bubble Tea model.
type App struct {
	ctx                *AppContext
	sidebar            sidebar.Model
	mainPanel          mainpanel.Model
	statusBar          statusbar.Model
	modal              modal.Model
	layout             Layout
	focus              PanelID
	ready              bool
	showHelp           bool
	activeFilter       string        // current filter: "My Issues", "All Issues", "Active", "Backlog"
	pendingIssue       *linear.Issue // issue awaiting workflow states for status change
	pendingEditIssue   *linear.Issue // issue awaiting metadata for edit modal
	pendingCreateIssue bool          // whether we are waiting for metadata to create an issue
}

// NewApp creates a new root App model.
func NewApp(client *linear.Client, state *config.State) App {
	ctx := &AppContext{
		Client: client,
	}

	sb := sidebar.New()
	sb.SetFocused(true)

	if state != nil {
		sb.SetInitialState(state.LastTeamID, state.LastFilter)
	}

	mp := mainpanel.New()
	mp.SetFocused(false)

	filter := "All Issues"
	if state != nil {
		if state.LastFilter != "" {
			filter = state.LastFilter
		}
		mp.SetCompact(state.CompactMode)
	}
	mp.SetFilterName(filter)

	return App{
		ctx:          ctx,
		sidebar:      sb,
		mainPanel:    mp,
		statusBar:    statusbar.New(),
		modal:        modal.New(),
		focus:        PanelSidebar,
		activeFilter: filter,
	}
}

// Init implements tea.Model. Fetches viewer and teams on startup.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.sidebar.Init(),
		a.fetchViewer(),
		a.fetchTeams(),
	)
}

// CurrentTeamID returns the currently selected team ID.
func (a App) CurrentTeamID() string {
	if a.ctx.CurrentTeam != nil {
		return a.ctx.CurrentTeam.ID
	}
	return ""
}

// CurrentFilter returns the currently selected filter.
func (a App) CurrentFilter() string {
	return a.activeFilter
}

// IsCompact returns whether the list is in compact mode.
func (a App) IsCompact() bool {
	return a.mainPanel.IsCompact()
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.ctx.Width = msg.Width
		a.ctx.Height = msg.Height
		a.layout = ComputeLayout(msg.Width, msg.Height)
		a.sidebar.SetSize(a.layout.SidebarWidth, a.layout.ContentHeight)
		a.mainPanel.SetSize(a.layout.MainWidth, a.layout.ContentHeight)
		a.statusBar.SetSize(msg.Width)
		a.modal.SetSize(msg.Width, msg.Height)
		if !a.ready {
			a.updateStatusBarHints()
		}
		a.ready = true
		return a, nil

	case tea.KeyPressMsg:
		// Always allow ctrl+c to quit.
		if msg.String() == KeyCtrlC {
			return a, tea.Quit
		}

		// If help overlay is shown, any key closes it.
		if a.showHelp {
			a.showHelp = false
			return a, nil
		}

		// When a modal is active, route all keys to it.
		if a.modal.Active() {
			var cmd tea.Cmd
			a.modal, cmd = a.modal.Update(msg)
			return a, cmd
		}

		switch msg.String() {
		case KeyQuit:
			// Don't quit if the main panel is filtering (user is typing).
			if a.focus == PanelMain && a.mainPanel.IsFiltering() {
				return a.routeKeyToFocused(msg)
			}
			return a, tea.Quit
		case KeyQuestionMark:
			// Don't open help if main panel is filtering (user is typing).
			if a.focus == PanelMain && a.mainPanel.IsFiltering() {
				return a.routeKeyToFocused(msg)
			}
			a.showHelp = true
			return a, nil
		case KeyTab:
			a.cycleFocus(1)
			a.updateStatusBarHints()
			return a, nil
		case KeyShiftTab:
			a.cycleFocus(-1)
			a.updateStatusBarHints()
			return a, nil
		case "c":
			// Open create issue globally, even if sidebar is focused
			if !a.mainPanel.IsFiltering() {
				return a, func() tea.Msg {
					return OpenCreateIssueMsg{}
				}
			}
			return a.routeKeyToFocused(msg)
		case "v":
			// Toggle compact mode globally, even if sidebar is focused
			if !a.mainPanel.IsFiltering() {
				a.mainPanel.ToggleCompact()
				return a, nil
			}
			return a.routeKeyToFocused(msg)
		default:
			// Route to focused panel.
			return a.routeKeyToFocused(msg)
		}

	case spinner.TickMsg:
		// Forward spinner ticks to both sidebar and main panel.
		updatedSidebar, cmd1 := a.sidebar.Update(msg)
		a.sidebar = updatedSidebar.(sidebar.Model)
		updatedMain, cmd2 := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		return a, tea.Batch(cmd1, cmd2)

	case ViewerLoadedMsg:
		user := msg.User
		a.ctx.CurrentUser = &user
		return a, nil

	case TeamsLoadedMsg:
		a.ctx.Teams = msg.Teams
		// Forward to sidebar.
		var cmd tea.Cmd
		updatedSidebar, cmd := a.sidebar.Update(msg)
		a.sidebar = updatedSidebar.(sidebar.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case TeamSelectedMsg:
		team := msg.Team
		a.ctx.CurrentTeam = &team
		// Forward to main panel and status bar.
		updatedMain, cmd1 := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		updatedStatus, cmd2 := a.statusBar.Update(msg)
		a.statusBar = updatedStatus.(statusbar.Model)
		if cmd1 != nil {
			cmds = append(cmds, cmd1)
		}
		if cmd2 != nil {
			cmds = append(cmds, cmd2)
		}
		// Fetch issues for the newly selected team with the active filter.
		cmds = append(cmds, a.fetchIssues(team.ID, a.activeFilter))
		// Also fetch team metadata to update sidebar filters
		cmds = append(cmds, a.fetchTeamMetadata(team.ID))
		a.updateStatusBarHints()
		return a, tea.Batch(cmds...)

	case FilterSelectedMsg:
		a.activeFilter = msg.Filter
		// Update status bar with filter context.
		a.statusBar.SetFilter(msg.Filter)
		a.mainPanel.SetFilterName(msg.Filter)
		// Re-fetch issues with the new filter if we have a team.
		if a.ctx.CurrentTeam != nil {
			updatedMain, cmd := a.mainPanel.Update(msg)
			a.mainPanel = updatedMain.(mainpanel.Model)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			cmds = append(cmds, a.fetchIssues(a.ctx.CurrentTeam.ID, a.activeFilter))
		}
		return a, tea.Batch(cmds...)

	case FilterCountsMsg:
		a.sidebar.SetFilterCounts(msg.Counts)
		return a, nil

	case IssuesLoadedMsg:
		// Forward to main panel.
		updatedMain, cmd := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case AutoTagIssuesMsg:
		return a, a.autoTagIssues(msg.Issues)

	case RefreshIssuesMsg:
		updatedMain, cmd := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, a.fetchIssues(a.ctx.CurrentTeam.ID, a.activeFilter))
		}
		return a, tea.Batch(cmds...)

	case FocusSidebarMsg:
		a.focus = PanelSidebar
		a.sidebar.SetFocused(true)
		a.mainPanel.SetFocused(false)
		a.updateStatusBarHints()
		return a, nil

	case FocusMainPanelMsg:
		a.focus = PanelMain
		a.sidebar.SetFocused(false)
		a.mainPanel.SetFocused(true)
		a.updateStatusBarHints()
		return a, nil

	// --- Modal triggers ---

	case OpenStatusChangeMsg:
		// Store the issue and fetch workflow states for the team.
		issue := msg.Issue
		a.pendingIssue = &issue
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, a.fetchWorkflowStates(a.ctx.CurrentTeam.ID))
		}
		return a, tea.Batch(cmds...)

	case WorkflowStatesLoadedMsg:
		// Open the status change modal with the fetched states.
		if a.pendingIssue != nil {
			a.modal.OpenStatusChange(*a.pendingIssue, msg.States)
			a.pendingIssue = nil
		}
		return a, nil

	case OpenCreateIssueMsg:
		// Open modal immediately, then lazy-load lists.
		a.statusBar.ClearSuccess()
		if a.ctx.CurrentTeam != nil {
			a.modal.OpenCreateIssue(a.ctx.CurrentTeam.ID, a.ctx.CurrentUser)
			a.pendingCreateIssue = true
			cmds = append(cmds, a.fetchTeamMetadata(a.ctx.CurrentTeam.ID))
		}
		return a, tea.Batch(cmds...)

	case OpenEditIssueMsg:
		issue := msg.Issue
		a.pendingEditIssue = &issue
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, a.fetchTeamMetadata(a.ctx.CurrentTeam.ID))
		}
		return a, tea.Batch(cmds...)

	case TeamMetadataLoadedMsg:
		if a.ctx.CurrentTeam != nil {
			if a.pendingEditIssue != nil {
				a.modal.OpenEditIssue(*a.pendingEditIssue, a.ctx.CurrentUser, msg.Metadata)
				a.pendingEditIssue = nil
			} else if a.pendingCreateIssue {
				a.modal.SetCreateIssueMetadata(msg.Metadata)
				a.pendingCreateIssue = false
			}

			if msg.Metadata != nil {
				a.ctx.CurrentProjects = msg.Metadata.Projects

				filters := []string{
					"My Issues",
					"My Unlabeled Issues",
					"My Issues + Active",
					"My Issues + Backlog",
				}

				if a.ctx.CurrentUser != nil {
					var projectFilters []string
					for _, p := range msg.Metadata.Projects {
						status := strings.ToLower(p.Status.Name)
						if status == "developing" && p.Lead != nil && p.Lead.ID == a.ctx.CurrentUser.ID {
							projectName := formatProjectNameForFilter(p.Name)
							projectFilters = append(projectFilters, projectName)
							projectFilters = append(projectFilters, projectName+" + Active")
							projectFilters = append(projectFilters, projectName+" + Backlog")
						}
					}
					if len(projectFilters) > 0 {
						filters = append(filters, "---")
						filters = append(filters, projectFilters...)
					}
				}

				filters = append(filters, "---")
				filters = append(filters, "All Issues", "Active", "Backlog")

				a.sidebar.SetFilters(filters)

				// Fetch issue counts for all non-separator filters.
				if a.ctx.CurrentTeam != nil {
					cmds = append(cmds, a.fetchFilterCounts(a.ctx.CurrentTeam.ID, filters))
				}
			}

			// Forward TeamMetadataLoadedMsg to sidebar to update filters
			updatedSidebar, cmd := a.sidebar.Update(msg)
			a.sidebar = updatedSidebar.(sidebar.Model)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
		return a, nil

	case ModalClosedMsg:
		a.modal.Close()
		return a, nil

	// --- Mutation confirmations from modals ---

	case modal.StatusChangeConfirmedMsg:
		a.modal.Close()
		cmds = append(cmds, a.updateIssueStatus(msg.IssueID, msg.NewStateID))
		return a, tea.Batch(cmds...)

	case modal.IssueCreateConfirmedMsg:
		a.modal.Close()
		a.statusBar.SetSuccess("Creating issue...")
		cmds = append(cmds, a.createIssue(msg))
		return a, tea.Batch(cmds...)

	case IssueEditConfirmedMsg:
		a.modal.Close()
		cmds = append(cmds, a.editIssue(msg))
		return a, tea.Batch(cmds...)

	// --- Mutation results ---

	case IssueUpdatedMsg:
		// Refresh the issue list with current filter.
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, a.fetchIssues(a.ctx.CurrentTeam.ID, a.activeFilter))
		}
		return a, tea.Batch(cmds...)

	case IssueCreatedMsg:
		a.statusBar.SetSuccess(fmt.Sprintf("Issue %s created successfully", msg.Issue.Identifier))
		// Refresh the issue list with current filter.
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, a.fetchIssues(a.ctx.CurrentTeam.ID, a.activeFilter))
		}
		return a, tea.Batch(cmds...)

	case IssueSelectedMsg, BackToListMsg:
		// Forward to main panel.
		updatedMain, cmd := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case ErrorMsg:
		// Forward to status bar.
		updatedStatus, cmd := a.statusBar.Update(msg)
		a.statusBar = updatedStatus.(statusbar.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Forward to sidebar (in case teams failed to load).
		updatedSidebar, cmd2 := a.sidebar.Update(msg)
		a.sidebar = updatedSidebar.(sidebar.Model)
		if cmd2 != nil {
			cmds = append(cmds, cmd2)
		}
		// Forward to main panel (in case issues failed to load).
		updatedMain, cmd3 := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		if cmd3 != nil {
			cmds = append(cmds, cmd3)
		}
		return a, tea.Batch(cmds...)
	}

	// Forward unhandled messages to the main panel so internal messages
	// like list.FilterMatchesMsg reach the list component.
	updatedMain, cmd := a.mainPanel.Update(msg)
	a.mainPanel = updatedMain.(mainpanel.Model)
	return a, cmd
}

// View implements tea.Model.
func (a App) View() tea.View {
	if !a.ready {
		return tea.NewView("Loading lazylinear...")
	}

	// If help overlay is shown, render it.
	if a.showHelp {
		v := tea.NewView(a.renderHelp())
		v.AltScreen = true
		return v
	}

	// If a modal is active, render it as a full-screen overlay.
	if a.modal.Active() {
		v := tea.NewView(a.modal.View())
		v.AltScreen = true
		return v
	}

	sidebarView := a.sidebar.View()
	mainView := a.mainPanel.View()
	statusView := a.statusBar.View()

	// Compose horizontal: sidebar | main.
	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebarView.Content,
		mainView.Content,
	)

	// Compose vertical: panels / status bar.
	full := lipgloss.JoinVertical(lipgloss.Left,
		topRow,
		statusView.Content,
	)

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

// cycleFocus switches focus between panels.
func (a *App) cycleFocus(direction int) {
	if direction > 0 {
		if a.focus == PanelSidebar {
			a.focus = PanelMain
		} else {
			a.focus = PanelSidebar
		}
	} else {
		if a.focus == PanelMain {
			a.focus = PanelSidebar
		} else {
			a.focus = PanelSidebar
		}
	}

	a.sidebar.SetFocused(a.focus == PanelSidebar)
	a.mainPanel.SetFocused(a.focus == PanelMain)
}

// routeKeyToFocused forwards a key press to the currently focused panel.
func (a App) routeKeyToFocused(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.focus {
	case PanelSidebar:
		updated, c := a.sidebar.Update(msg)
		a.sidebar = updated.(sidebar.Model)
		cmd = c
	case PanelMain:
		updated, c := a.mainPanel.Update(msg)
		a.mainPanel = updated.(mainpanel.Model)
		cmd = c
	}
	return a, cmd
}

// fetchViewer returns a command that fetches the authenticated user.
func (a App) fetchViewer() tea.Cmd {
	return func() tea.Msg {
		viewer, err := a.ctx.Client.GetViewer()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch viewer: %w", err)}
		}
		return ViewerLoadedMsg{User: *viewer}
	}
}

// fetchTeams returns a command that fetches the user's teams.
func (a App) fetchTeams() tea.Cmd {
	return func() tea.Msg {
		teams, err := a.ctx.Client.GetTeams()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch teams: %w", err)}
		}
		return TeamsLoadedMsg{Teams: teams}
	}
}

// fetchIssues returns a command that fetches issues for the given team with an optional status filter.
func (a App) fetchIssues(teamID string, filterName string) tea.Cmd {
	filter := buildIssueFilter(filterName, a.ctx.CurrentUser, a.ctx.CurrentProjects)
	return func() tea.Msg {
		conn, err := a.ctx.Client.GetIssues(teamID, 50, "", filter)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch issues: %w", err)}
		}
		return IssuesLoadedMsg{
			Issues:   conn.Nodes,
			PageInfo: conn.PageInfo,
		}
	}
}

// fetchFilterCounts returns a command that fetches issue counts for all non-separator filters.
func (a App) fetchFilterCounts(teamID string, filterNames []string) tea.Cmd {
	filterMap := make(map[string]map[string]any)
	for _, name := range filterNames {
		if name == "---" {
			continue
		}
		f := buildIssueFilter(name, a.ctx.CurrentUser, a.ctx.CurrentProjects)
		filterMap[name] = f
	}
	return func() tea.Msg {
		counts, err := a.ctx.Client.GetFilterCounts(teamID, filterMap)
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
				"and": []map[string]any{
					{
						"assignee": map[string]any{
							"id": map[string]any{"eq": currentUser.ID},
						},
					},
					{
						"labels": map[string]any{
							"null": true,
						},
					},
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
	case "My Issues + Backlog":
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
							"type": map[string]any{"in": []string{"backlog", "triage"}},
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
	case "Backlog":
		return map[string]any{
			"state": map[string]any{
				"type": map[string]any{"in": []string{"backlog", "triage"}},
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
		if filterName == projectName+" + Backlog" {
			return map[string]any{
				"and": []map[string]any{
					{
						"project": map[string]any{
							"id": map[string]any{"eq": p.ID},
						},
					},
					{
						"state": map[string]any{
							"type": map[string]any{"in": []string{"backlog", "triage"}},
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
func (a App) fetchWorkflowStates(teamID string) tea.Cmd {
	return func() tea.Msg {
		states, err := a.ctx.Client.GetWorkflowStates(teamID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch workflow states: %w", err)}
		}
		return WorkflowStatesLoadedMsg{States: states}
	}
}

// fetchTeamMetadata returns a command that fetches team metadata (members, projects, cycles).
func (a App) fetchTeamMetadata(teamID string) tea.Cmd {
	return func() tea.Msg {
		meta, err := a.ctx.Client.GetTeamMetadata(teamID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch team metadata: %w", err)}
		}
		return TeamMetadataLoadedMsg{Metadata: meta}
	}
}

// updateIssueStatus returns a command that updates an issue's workflow state.
func (a App) updateIssueStatus(issueID, stateID string) tea.Cmd {
	return func() tea.Msg {
		updated, err := a.ctx.Client.UpdateIssue(issueID, linear.IssueUpdateInput{
			StateID: &stateID,
		})
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("update issue status: %w", err)}
		}
		return IssueUpdatedMsg{Issue: *updated}
	}
}

// updateStatusBarHints sets the status bar key hints based on the current state.
func (a *App) updateStatusBarHints() {
	if a.modal.Active() {
		a.statusBar.SetHints("tab: fields | enter: submit | esc: cancel")
		return
	}
	switch a.focus {
	case PanelSidebar:
		a.statusBar.SetHints("j/k: navigate | enter: select | l: select & focus | c: create | v: compact | tab: issues | ?: help")
	case PanelMain:
		if a.mainPanel.Focused() {
			hints := "j/k: navigate | /: filter | enter: open | c: create | e: edit | s: status | v: compact | ?: help"
			if a.activeFilter == "My Unlabeled Issues" {
				hints = "T: auto-tag | " + hints
			}
			a.statusBar.SetHints(hints)
		} else {
			a.statusBar.SetHints("tab: teams | ?: help")
		}
	}
}

// renderHelp returns a full-screen help overlay showing all key bindings.
func (a App) renderHelp() string {
	width := a.ctx.Width
	height := a.ctx.Height

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666"))

	var b strings.Builder

	b.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	sections := []struct {
		title string
		keys  []struct{ key, desc string }
	}{
		{
			title: "Global",
			keys: []struct{ key, desc string }{
				{"ctrl+c", "quit"},
				{"q", "quit"},
				{"tab / shift+tab", "switch panel"},
				{"c", "create new issue"},
				{"v", "toggle compact view"},
				{"?", "toggle help"},
			},
		},
		{
			title: "Sidebar",
			keys: []struct{ key, desc string }{
				{"j / k", "navigate up/down"},
				{"enter / l", "select item"},
				{"g / G", "jump to top/bottom"},
			},
		},
		{
			title: "Issue List",
			keys: []struct{ key, desc string }{
				{"j / k", "navigate up/down"},
				{"/", "filter issues"},
				{"enter / l", "open issue detail"},
				{"e", "edit issue"},
				{"s", "change status"},
				{"r", "refresh list"},
				{"h", "focus sidebar"},
				{"T", "auto-tag (unlabeled only)"},
			},
		},
		{
			title: "Issue Detail",
			keys: []struct{ key, desc string }{
				{"j / k", "scroll up/down"},
				{"esc / h / q", "back to list"},
				{"e", "edit issue"},
				{"s", "change status"},
			},
		},
		{
			title: "Modals",
			keys: []struct{ key, desc string }{
				{"tab / shift+tab", "switch fields"},
				{"enter", "submit / confirm"},
				{"esc", "cancel / close"},
			},
		},
	}

	for _, sec := range sections {
		b.WriteString(sectionStyle.Render(sec.title))
		b.WriteString("\n")
		for _, k := range sec.keys {
			b.WriteString(fmt.Sprintf("  %s  %s\n",
				keyStyle.Width(20).Render(k.key),
				descStyle.Render(k.desc),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(dimStyle.Render("Press any key to close"))

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 3).
		Render(b.String())

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// createIssue returns a command that creates a new issue.
func (a App) createIssue(confirmed modal.IssueCreateConfirmedMsg) tea.Cmd {
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

		issue, err := a.ctx.Client.CreateIssue(input)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("create issue: %w", err)}
		}
		return IssueCreatedMsg{Issue: *issue}
	}
}

// editIssue returns a command that edits an existing issue.
func (a App) editIssue(confirmed IssueEditConfirmedMsg) tea.Cmd {
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

		issue, err := a.ctx.Client.UpdateIssue(confirmed.IssueID, input)
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
func (a App) autoTagIssues(issues []linear.Issue) tea.Cmd {
	if len(issues) == 0 {
		return nil
	}

	return func() tea.Msg {
		if a.ctx.CurrentTeam == nil {
			return ErrorMsg{Err: fmt.Errorf("no current team")}
		}

		meta, err := a.ctx.Client.GetTeamMetadata(a.ctx.CurrentTeam.ID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("fetch team metadata: %w", err)}
		}

		labelMap := make(map[string]string)
		for _, l := range meta.Labels {
			labelMap[l.Name] = l.ID
		}

		// Ensure all allowed labels exist or skip those that don't
		existingAllowed := []string{}
		for _, name := range allowedLabels {
			if _, ok := labelMap[name]; ok {
				existingAllowed = append(existingAllowed, name)
			}
		}

		if len(existingAllowed) == 0 {
			return ErrorMsg{Err: fmt.Errorf("none of the allowed labels exist in this team")}
		}

		// 2. Prepare prompt
		var issuesText strings.Builder
		for _, issue := range issues {
			desc := issue.Description
			if len(desc) > 300 {
				desc = desc[:300]
			}
			fmt.Fprintf(&issuesText, "ID: %s\nTitle: %s\nDescription: %s\n---\n", issue.Identifier, issue.Title, desc)
		}

		prompt := fmt.Sprintf(
			"Categorize the following Linear issues into EXACTLY ONE of these categories:\n%s\n\nIssues:\n%s\n\nRespond ONLY with a list of \"ID: Category\", one per line. Do not include any other text or markdown.",
			strings.Join(existingAllowed, ", "),
			issuesText.String(),
		)

		// 3. Run gemini CLI
		cmd := exec.Command("gemini", "-p", prompt)
		output, err := cmd.Output()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("run gemini cli: %w", err)}
		}

		// 4. Parse output
		suggestions := make(map[string]string)
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if parts := strings.Split(line, ":"); len(parts) >= 2 {
				id := strings.TrimSpace(parts[0])
				category := strings.TrimSpace(parts[1])
				for _, allowed := range existingAllowed {
					if strings.Contains(strings.ToLower(category), strings.ToLower(allowed)) {
						suggestions[id] = labelMap[allowed]
						break
					}
				}
			}
		}

		// 5. Update issues
		for _, issue := range issues {
			if labelID, ok := suggestions[issue.Identifier]; ok {
				if err := a.ctx.Client.UpdateIssueLabels(issue.ID, []string{labelID}); err != nil {
					// continue on error
				}
			}
		}

		return RefreshIssuesMsg{}
	}
}
