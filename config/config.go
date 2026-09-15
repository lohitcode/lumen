// Package config persists user-level state across sessions: which light was
// last opened, so the next launch reconnects to it automatically.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// LastLight identifies the light from the previous session, plus any
// display name the user gave it.
type LastLight struct {
	Driver  string `json:"driver"`
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type file struct {
	Last *LastLight `json:"last_light"`
}

// LoadLast returns the light saved by the most recent session, if any.
func LoadLast() (LastLight, bool) {
	data, err := os.ReadFile(path())
	if err != nil {
		return LastLight{}, false
	}
	var parsed file
	if json.Unmarshal(data, &parsed) != nil || parsed.Last == nil {
		return LastLight{}, false
	}
	return *parsed.Last, true
}

// SaveLast records the light to reopen on the next launch.
func SaveLast(l LastLight) error {
	if l.Driver == "" || l.Address == "" {
		return errors.New("refusing to save an incomplete light reference")
	}
	data, err := json.MarshalIndent(file{Last: &l}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path(), data, 0o600)
}

func path() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "lumen", "config.json")
}
