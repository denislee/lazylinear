package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	ctx              *AppContext
	sidebar          sidebar.Model
	mainPanel        mainpanel.Model
	statusBar        statusbar.Model
	modal            modal.Model
	layout           Layout
	focus            PanelID
	ready            bool
	showHelp         bool
	activeFilter     string        // current filter: "My Issues", "All Issues", "Active", "Backlog"
	pendingIssue     *linear.Issue // issue awaiting workflow states for status change
}

// NewApp creates a new root App model.
func NewApp(client *linear.Client) App {
	ctx := &AppContext{
		Client: client,
	}

	sb := sidebar.New()
	sb.SetFocused(true)

	mp := mainpanel.New()
	mp.SetFocused(false)

	return App{
		ctx:          ctx,
		sidebar:      sb,
		mainPanel:    mp,
		statusBar:    statusbar.New(),
		modal:        modal.New(),
		focus:        PanelSidebar,
		activeFilter: "All Issues",
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
		a.updateStatusBarHints()
		return a, tea.Batch(cmds...)

	case FilterSelectedMsg:
		a.activeFilter = msg.Filter
		// Update status bar with filter context.
		a.statusBar.SetFilter(msg.Filter)
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

	case IssuesLoadedMsg:
		// Forward to main panel.
		updatedMain, cmd := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	case RefreshIssuesMsg:
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
		// Open the create issue modal for the current team.
		if a.ctx.CurrentTeam != nil {
			a.modal.OpenCreateIssue(a.ctx.CurrentTeam.ID)
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
		cmds = append(cmds, a.createIssue(msg))
		return a, tea.Batch(cmds...)

	// --- Mutation results ---

	case IssueUpdatedMsg:
		// Refresh the issue list with current filter.
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, a.fetchIssues(a.ctx.CurrentTeam.ID, a.activeFilter))
		}
		return a, tea.Batch(cmds...)

	case IssueCreatedMsg:
		// Refresh the issue list with current filter.
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, a.fetchIssues(a.ctx.CurrentTeam.ID, a.activeFilter))
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

	return a, nil
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
			a.focus = PanelMain
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
	filter := buildIssueFilter(filterName, a.ctx.CurrentUser)
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

// buildIssueFilter converts a sidebar filter name to a Linear GraphQL IssueFilter.
func buildIssueFilter(filterName string, currentUser *linear.User) map[string]any {
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
	default:
		return nil
	}
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
		a.statusBar.SetHints("j/k: navigate | enter: select | tab: issues | ?: help")
	case PanelMain:
		if a.ctx.CurrentTeam != nil {
			a.statusBar.SetHints("j/k: navigate | /: filter | enter: open | c: create | s: status | ?: help")
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
				{"c", "create new issue"},
				{"s", "change status"},
				{"r", "refresh list"},
				{"h", "focus sidebar"},
			},
		},
		{
			title: "Issue Detail",
			keys: []struct{ key, desc string }{
				{"j / k", "scroll up/down"},
				{"esc / h / q", "back to list"},
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
			p := confirmed.Priority
			input.Priority = &p
		}
		created, err := a.ctx.Client.CreateIssue(input)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("create issue: %w", err)}
		}
		return IssueCreatedMsg{Issue: *created}
	}
}
