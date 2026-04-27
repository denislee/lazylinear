package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type State struct {
	LastTeamID  string `json:"last_team_id"`
	LastFilter  string `json:"last_filter"`
	CompactMode bool   `json:"compact_mode"`
}

func defaultStatePath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "lazylinear", "state.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lazylinear", "state.json")
}

// LoadState reads persisted UI state from disk. A missing or corrupt file
// falls back to defaults — persisted state is a convenience, not critical.
func LoadState() *State {
	state := &State{}
	if data, err := os.ReadFile(defaultStatePath()); err == nil {
		// Ignore unmarshal error: fall back to defaults on corruption.
		_ = json.Unmarshal(data, state)
	}
	if state.LastFilter == "" {
		state.LastFilter = "My Issues + Active"
	}
	return state
}

func SaveState(state *State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	path := defaultStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}
