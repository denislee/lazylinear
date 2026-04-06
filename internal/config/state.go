package config

import (
	"encoding/json"
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

func LoadState() *State {
	state := &State{}
	data, err := os.ReadFile(defaultStatePath())
	if err == nil {
		json.Unmarshal(data, state)
	}
	if state.LastFilter == "" {
		state.LastFilter = "All Issues"
	}
	return state
}

func SaveState(state *State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := defaultStatePath()
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, data, 0644)
}
