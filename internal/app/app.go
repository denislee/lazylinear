package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/denislee/lazylinear/internal/config"
	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
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
	ctx                 *AppContext
	sidebar             sidebar.Model
	mainPanel           mainpanel.Model
	statusBar           statusbar.Model
	modal               modal.Model
	layout              Layout
	focus               PanelID
	ready               bool
	showHelp            bool
	activeFilter        string        // current filter: "My Issues", "All Issues", "Active"
	pendingIssue        *linear.Issue // issue awaiting workflow states for status change
	pendingEditIssue    *linear.Issue // issue awaiting metadata for edit modal
	pendingCreateIssue  bool          // whether we are waiting for metadata to create an issue
	autoLabelingIssues  []linear.Issue
	autoLabelingIndex   int
	autoLabelingMap     map[string]string
	autoLabelingAllowed []string
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
		fetchViewer(a.ctx),
		fetchTeams(a.ctx),
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a.handleWindowSizeMsg(msg)
	case tea.KeyPressMsg:
		return a.handleKeyPressMsg(msg)
	default:
		return a.handleCustomMsg(msg)
	}
}

func (a App) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
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

	// Forward resize to modal as well, so internal components can update
	if a.modal.Active() {
		var cmd tea.Cmd
		a.modal, cmd = a.modal.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Let the fallback handle the rest of the message so the background updates
	updatedMain, mainCmd := a.mainPanel.Update(msg)
	a.mainPanel = updatedMain.(mainpanel.Model)
	if mainCmd != nil {
		cmds = append(cmds, mainCmd)
	}
	return a, tea.Batch(cmds...)
}

func (a App) handleKeyPressMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case KeyCtrlK:
		if !a.mainPanel.IsFiltering() {
			return a, func() tea.Msg {
				return OpenIssueSearchMsg{}
			}
		}
		return a.routeKeyToFocused(msg)
	default:
		// Route to focused panel.
		return a.routeKeyToFocused(msg)
	}
}

func (a App) handleCustomMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
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
		cmds = append(cmds, fetchIssues(a.ctx, team.ID, a.activeFilter))
		// Also fetch team metadata to update sidebar filters
		cmds = append(cmds, fetchTeamMetadata(a.ctx, team.ID))
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
			cmds = append(cmds, fetchIssues(a.ctx, a.ctx.CurrentTeam.ID, a.activeFilter))
		}
		return a, tea.Batch(cmds...)

	case FilterCountsMsg:
		a.sidebar.SetFilterCounts(msg.Counts)
		return a, nil

	case IssuesLoadedMsg:
		// Only apply the message if it corresponds to the currently active filter.
		// This prevents issues from a previously selected filter from overwriting
		// the results of a more recent selection due to race conditions.
		if msg.FilterName != a.activeFilter {
			return a, nil
		}
		// Forward to main panel.
		updatedMain, cmd := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		return a, cmd

	case AutoTagIssuesMsg:
		a.statusBar.SetSuccess("Auto-labeling: Fetching metadata...")
		return a, autoTagIssues(a.ctx, msg.Issues)

	case appmsg.AutoLabelStartMsg:
		a.autoLabelingIssues = msg.Issues
		a.autoLabelingMap = msg.LabelMap
		a.autoLabelingAllowed = msg.Allowed
		a.autoLabelingIndex = 0
		a.statusBar.SetSuccess(fmt.Sprintf("Auto-labeling [0/%d]: Preparing...", len(a.autoLabelingIssues)))
		if a.autoLabelingIndex >= len(a.autoLabelingIssues) {
			return a, func() tea.Msg { return RefreshIssuesMsg{} }
		}
		return a, processNextIssue(a.ctx, a.autoLabelingIssues[a.autoLabelingIndex], a.autoLabelingIndex+1, len(a.autoLabelingIssues), a.autoLabelingAllowed, a.autoLabelingMap)

	case appmsg.AutoLabelProgressMsg:
		a.statusBar.SetSuccess(msg.Message)
		a.autoLabelingIndex++
		if a.autoLabelingIndex >= len(a.autoLabelingIssues) {
			return a, func() tea.Msg { return RefreshIssuesMsg{} }
		}
		return a, processNextIssue(a.ctx, a.autoLabelingIssues[a.autoLabelingIndex], a.autoLabelingIndex+1, len(a.autoLabelingIssues), a.autoLabelingAllowed, a.autoLabelingMap)

	case RefreshIssuesMsg:
		a.statusBar.SetSuccess("Auto-labeling complete")
		updatedMain, cmd := a.mainPanel.Update(msg)
		a.mainPanel = updatedMain.(mainpanel.Model)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, fetchIssues(a.ctx, a.ctx.CurrentTeam.ID, a.activeFilter))
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
			cmds = append(cmds, fetchWorkflowStates(a.ctx, a.ctx.CurrentTeam.ID))
		}
		return a, tea.Batch(cmds...)

	case OpenIssueSearchMsg:
		a.statusBar.SetSuccess("Fetching my issues...")
		return a, fetchMyIssues(a.ctx)

	case MyIssuesLoadedMsg:
		a.statusBar.ClearSuccess()
		a.modal.OpenIssueSearch(msg.Issues)
		return a, nil

	case modal.IssueSearchConfirmedMsg:
		a.modal.Close()
		// Navigate to the selected issue.
		// Since we might be in a different team than the issue, we might need to switch team.
		// For now, let's just select the issue in the current panel.
		// Actually, if we are in search, we want to see it.
		return a, func() tea.Msg {
			return IssueSelectedMsg{Issue: msg.Issue}
		}

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
			cmds = append(cmds, fetchTeamMetadata(a.ctx, a.ctx.CurrentTeam.ID))
		}
		return a, tea.Batch(cmds...)

	case OpenEditIssueMsg:
		issue := msg.Issue
		a.pendingEditIssue = &issue
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, fetchTeamMetadata(a.ctx, a.ctx.CurrentTeam.ID))
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
					"My Issues + Active",
					"My Unlabeled Issues",
				}

				if a.ctx.CurrentUser != nil {
					var projectFilters []string
					for _, p := range msg.Metadata.Projects {
						status := strings.ToLower(p.Status.Name)
						if status == "developing" && p.Lead != nil && p.Lead.ID == a.ctx.CurrentUser.ID {
							projectName := formatProjectNameForFilter(p.Name)
							projectFilters = append(projectFilters, projectName)
							projectFilters = append(projectFilters, projectName+" + Active")
						}
					}
					if len(projectFilters) > 0 {
						filters = append(filters, "---")
						filters = append(filters, projectFilters...)
					}
				}

				filters = append(filters, "---")
				filters = append(filters, "All Issues", "Active")

				a.sidebar.SetFilters(filters)

				// Fetch issue counts for all non-separator filters.
				if a.ctx.CurrentTeam != nil {
					cmds = append(cmds, fetchFilterCounts(a.ctx, a.ctx.CurrentTeam.ID, filters))
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
		cmds = append(cmds, updateIssueStatus(a.ctx, msg.IssueID, msg.NewStateID))
		return a, tea.Batch(cmds...)

	case modal.IssueCreateConfirmedMsg:
		a.modal.Close()
		a.statusBar.SetSuccess("Creating issue...")
		cmds = append(cmds, createIssue(a.ctx, msg))
		return a, tea.Batch(cmds...)

	case IssueEditConfirmedMsg:
		a.modal.Close()
		cmds = append(cmds, editIssue(a.ctx, msg))
		return a, tea.Batch(cmds...)

	// --- Mutation results ---

	case IssueUpdatedMsg:
		// Refresh the issue list with current filter.
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, fetchIssues(a.ctx, a.ctx.CurrentTeam.ID, a.activeFilter))
		}
		return a, tea.Batch(cmds...)

	case IssueCreatedMsg:
		a.statusBar.SetSuccess(fmt.Sprintf("Issue %s created successfully", msg.Issue.Identifier))
		// Refresh the issue list with current filter.
		if a.ctx.CurrentTeam != nil {
			cmds = append(cmds, fetchIssues(a.ctx, a.ctx.CurrentTeam.ID, a.activeFilter))
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

	case OpenIssueInBrowserMsg:
		return a, openBrowser(msg.Issue.URL)

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

	// When a modal is active, forward unhandled messages (including cursor
	// blink ticks) to it so internal components keep working.
	if a.modal.Active() {
		var cmd tea.Cmd
		a.modal, cmd = a.modal.Update(msg)
		return a, cmd
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

// updateStatusBarHints sets the status bar key hints based on the current state.
func (a *App) updateStatusBarHints() {
	if a.modal.Active() {
		a.statusBar.SetHints("tab: fields | enter: submit | esc: cancel")
		return
	}
	switch a.focus {
	case PanelSidebar:
		a.statusBar.SetHints("j/k: navigate | enter: select | l: select & focus | c: create | ctrl+k: search | tab: issues | ?: help")
	case PanelMain:
		if a.mainPanel.Focused() {
			hints := "j/k: navigate | /: filter | enter: browser | l: open | c: create | ctrl+k: search | ?: help"
			if a.activeFilter == "My Unlabeled Issues" {
				hints = "t: auto-label | " + hints
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
				{"ctrl+k", "search my issues"},
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
				{"enter", "open in browser"},
				{"l", "open issue detail"},
				{"e", "edit issue"},
				{"s", "change status"},
				{"r", "refresh list"},
				{"h", "focus sidebar"},
				{"t", "auto-label (unlabeled only)"},
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
