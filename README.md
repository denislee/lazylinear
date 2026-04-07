# Lazylinear

Lazylinear is a fast, keyboard-driven Terminal User Interface (TUI) client for [Linear.app](https://linear.app), built in Go using the [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) framework. It allows you to seamlessly browse, create, edit, and manage the status of your issues directly from your terminal.

## Features

- **Fast & Keyboard-Driven:** Navigate your Linear workspace efficiently without leaving your terminal.
- **Issue Management:** Browse, create, edit, and change the status of issues.
- **Dynamic Filtering:** Filter issues by team, project, status, and more.
- **Custom UI:** Built with Charm's Bubble Tea, Bubbles, and Lipgloss for a beautiful and responsive terminal experience.

## Technologies Used

- **Go:** Primary language.
- **Bubble Tea v2 (`charm.land/bubbletea/v2`):** Core TUI framework based on The Elm Architecture.
- **Bubbles (`charm.land/bubbles/v2`):** UI components for Bubble Tea.
- **Lipgloss (`charm.land/lipgloss/v2`):** Styling and layout for terminal applications.
- **GraphQL:** Used to interact directly with the Linear API.

## Installation & Setup

### Prerequisites

You need a Linear API key to use Lazylinear. You can generate one from your Linear account settings. 

Configure the API key in one of the following ways:
1. Set the `LAZYLINEAR_API_KEY` environment variable:
   ```bash
   export LAZYLINEAR_API_KEY="your_api_key_here"
   ```
2. Save it in the configuration file at `~/.config/lazylinear/config.yaml`:
   ```yaml
   api_key: "your_api_key_here"
   ```

### Building and Running

Ensure you have Go installed, then clone the repository and run:

```bash
# Run the application directly
go run main.go

# Build the executable binary
go build -o lazylinear
./lazylinear

# Run all tests
go test ./...
```

## Architecture Overview

Lazylinear adheres to the Bubble Tea model pattern, where UI components implement the `tea.Model` interface with `Init()`, `Update()`, and `View()` methods.

- **Root Orchestrator (`internal/app/app.go`):** The main Bubble Tea model that owns all panels, handles routing of messages, and manages focus state.
- **Shared Context (`internal/app/context.go`):** Holds global state, including the API client, current user, active team, projects, and dimensions.
- **Panel Hierarchy (`internal/panel/`):**
  - **Sidebar:** Handles team list and dynamic filter selections.
  - **Main:** Switches between list view and detail view.
  - **Modal:** Overlay forms for creating, editing issues, and changing status.
  - **Statusbar:** Contextual help hints, key bindings, and error displays.
- **API Layer (`internal/linear/`):** Custom GraphQL HTTP client.
- **Configuration & State (`internal/config/`):** Manages API keys and persists UI session state (last team, filter, compact mode).
- **Theming (`internal/theme/`):** Centralized Lipgloss styles and colors.

## Development

- **Message-Driven:** All async operations flow through Bubble Tea commands and custom messages (`internal/msg/msg.go`).
- **Focus Management:** Focus is maintained on a single panel at a time, switchable via `Tab`/`Shift+Tab`.
- **GraphQL:** Uses explicit JSON requests instead of bulky GraphQL client libraries.

## License

MIT
