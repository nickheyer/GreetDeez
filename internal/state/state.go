package state

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

const appName = "greetdeez"

// remembered across logins
type State struct {
	LastUser    string `json:"last_user,omitempty"`
	LastSession string `json:"last_session,omitempty"`
}

// Load reads state file missing or corrupt gives empty
func Load() State {
	path := filePath()
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Debug("no state file, using defaults", "path", path)
		return State{}
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		slog.Warn("corrupt state file, resetting", "path", path, "error", err)
		return State{}
	}
	return s
}

// Save writes state file atomically
func Save(s State) error {
	path := filePath()

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	// 0600 last user is nobodys business
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func filePath() string {
	if p := os.Getenv("GREETDEEZ_STATE_FILE"); p != "" {
		return p
	}
	return filepath.Join("/var/cache", appName, "state.json")
}
