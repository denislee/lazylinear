package app

// Layout holds the computed dimensions for each panel region.
type Layout struct {
	SidebarWidth    int
	MainWidth       int
	ContentHeight   int
	StatusBarHeight int
}

// ComputeLayout calculates panel dimensions from the terminal size.
// Sidebar = min(30, width/4). Main = width - sidebarWidth.
// ContentHeight = height - 1 (status bar). StatusBarHeight = 1.
func ComputeLayout(width, height int) Layout {
	sidebarWidth := max(10, min(30, width/4))

	// Main panel takes the rest of the available width.
	mainWidth := max(10, width-sidebarWidth)

	// Reserve space for status bar (1 line). The panels handle their own borders internally.
	contentHeight := max(3, height-1)

	return Layout{
		SidebarWidth:    sidebarWidth,
		MainWidth:       mainWidth,
		ContentHeight:   contentHeight,
		StatusBarHeight: 1,
	}
}
