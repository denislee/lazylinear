package app

// Layout holds the computed dimensions for each panel region.
type Layout struct {
	SidebarWidth    int
	MainWidth       int
	ContentHeight   int
	StatusBarHeight int
}

// ComputeLayout calculates panel dimensions from the terminal size.
// Sidebar = min(30, width/4). Main = width - sidebarWidth - 2 (borders).
// ContentHeight = height - 3 (status bar + borders). StatusBarHeight = 1.
func ComputeLayout(width, height int) Layout {
	sidebarWidth := max(10, min(30, width/4))

	// Account for borders on both sidebar and main panels (1 char each side).
	mainWidth := max(10, width-sidebarWidth-2)

	// Reserve space for status bar (1 line) and top/bottom borders (2 lines).
	contentHeight := max(3, height-3)

	return Layout{
		SidebarWidth:    sidebarWidth,
		MainWidth:       mainWidth,
		ContentHeight:   contentHeight,
		StatusBarHeight: 1,
	}
}
