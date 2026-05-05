package projectissues

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/denislee/lazylinear/internal/linear"
	appmsg "github.com/denislee/lazylinear/internal/msg"
	"github.com/denislee/lazylinear/internal/panel/main/issuelist"
)

type cycleItem struct {
	cycle  linear.Cycle
	issues []linear.Issue
}

func (i cycleItem) Title() string {
	if i.cycle.Name != "" {
		return fmt.Sprintf("Cycle %d: %s (%d issues)", i.cycle.Number, i.cycle.Name, len(i.issues))
	}
	return fmt.Sprintf("Cycle %d (%d issues)", i.cycle.Number, len(i.issues))
}

func (i cycleItem) Description() string {
	start := i.cycle.StartsAt.Format("Jan 02")
	end := i.cycle.EndsAt.Format("Jan 02, 2006")
	return fmt.Sprintf("%s - %s", start, end)
}

func (i cycleItem) FilterValue() string {
	return i.Title()
}

type Model struct {
	projectName string
	cycles      []linear.ProjectCycleIssues
	cycleList   list.Model
	issueList   issuelist.Model
	
	showIssues  bool
	width       int
	height      int
	focused     bool
}

func New() Model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Cycles"
	l.SetShowHelp(false)
	l.SetStatusBarItemName("cycle", "cycles")
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)

	il := issuelist.New()

	return Model{
		cycleList: l,
		issueList: il,
	}
}

func (m Model) ProjectName() string {
	return m.projectName
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.cycleList.SetSize(width, height)
	m.issueList.SetSize(width, height)
}

func (m *Model) SetFocused(focused bool) {
	m.focused = focused
	m.issueList.SetFocused(focused)
	if !focused {
		m.showIssues = false
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case appmsg.ProjectSelectedMsg:
		m.projectName = msg.Project.Name
		m.cycles = nil
		m.showIssues = false
		cmd := m.cycleList.SetItems(nil)
		m.cycleList.Title = "Cycles - " + m.projectName
		return m, cmd

	case appmsg.ProjectCyclesLoadedMsg:
		m.projectName = msg.ProjectName
		m.cycles = msg.Cycles
		m.showIssues = false
		
		items := make([]list.Item, len(msg.Cycles))
		for i, ci := range msg.Cycles {
			items[i] = cycleItem{cycle: ci.Cycle, issues: ci.Issues}
		}
		cmd := m.cycleList.SetItems(items)
		cmds = append(cmds, cmd)
		m.cycleList.Title = "Cycles - " + m.projectName
		return m, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}

		if m.showIssues {
			if msg.String() == "esc" || msg.String() == "h" {
				m.showIssues = false
				return m, nil
			}
			var cmd tea.Cmd
			m.issueList, cmd = m.issueList.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "enter", "l":
			if item, ok := m.cycleList.SelectedItem().(cycleItem); ok {
				m.showIssues = true
				m.issueList.SetFilterName(fmt.Sprintf("%s - %s", m.projectName, item.Title()))
				// Mock IssuesLoadedMsg to populate issueList
				updatedIssueList, cmd := m.issueList.Update(appmsg.IssuesLoadedMsg{
					Issues: item.issues,
				})
				m.issueList = updatedIssueList
				return m, cmd
			}
		case "y":
			if item, ok := m.cycleList.SelectedItem().(cycleItem); ok {
				return m, func() tea.Msg {
					return appmsg.CopyCycleIssuesMsg{
						Cycle:  item.cycle,
						Issues: item.issues,
					}
				}
			}
		case "h", "esc":
			return m, func() tea.Msg { return appmsg.FocusSidebarMsg{} }
		}

		var cmd tea.Cmd
		m.cycleList, cmd = m.cycleList.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.showIssues {
		return m.issueList.View()
	}
	return m.cycleList.View()
}
