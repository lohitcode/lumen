// Package config persists user-level state across sessions: which light was
// last opened, so the next launch reconnects to it automatically.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// LastLight identifies the light from the previous session. Name is only
// read for migrating configs saved before per-light names existed.
type LastLight struct {
	Driver  string `json:"driver"`
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type file struct {
	Last  *LastLight        `json:"last_light,omitempty"`
	Names map[string]string `json:"names,omitempty"`
}

// NamesKey builds the map key used to store a light's display name.
func NamesKey(driver, address string) string {
	return driver + "/" + address
}

// LoadNames returns the user-chosen display names keyed by light. Configs
// from before per-light names existed are migrated on first read.
func LoadNames() map[string]string {
	names := map[string]string{}
	parsed, ok := read()
	if !ok {
		return names
	}
	for k, v := range parsed.Names {
		names[k] = v
	}
	if parsed.Last != nil && parsed.Last.Name != "" {
		key := NamesKey(parsed.Last.Driver, parsed.Last.Address)
		if _, exists := names[key]; !exists {
			names[key] = parsed.Last.Name
			parsed.Names = names
			_ = write(parsed)
		}
	}
	return names
}

// SaveName stores (or, for an empty name, clears) a light's display name.
func SaveName(driver, address, name string) error {
	if driver == "" || address == "" {
		return errors.New("refusing to save a name for an incomplete light reference")
	}
	parsed, _ := read()
	if parsed.Names == nil {
		parsed.Names = map[string]string{}
	}
	key := NamesKey(driver, address)
	if name == "" {
		delete(parsed.Names, key)
	} else {
		parsed.Names[key] = name
	}
	return write(parsed)
}

// LoadLast returns the light saved by the most recent session, if any.
func LoadLast() (LastLight, bool) {
	parsed, ok := read()
	if !ok || parsed.Last == nil {
		return LastLight{}, false
	}
	return *parsed.Last, true
}

// SaveLast records the light to reopen on the next launch.
func SaveLast(l LastLight) error {
	if l.Driver == "" || l.Address == "" {
		return errors.New("refusing to save an incomplete light reference")
	}
	parsed, _ := read()
	parsed.Last = &l
	return write(parsed)
}

func read() (file, bool) {
	data, err := os.ReadFile(path())
	if err != nil {
		return file{}, false
	}
	var parsed file
	if json.Unmarshal(data, &parsed) != nil {
		return file{}, false
	}
	return parsed, true
}

func write(f file) error {
	data, err := json.MarshalIndent(f, "", "  ")
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
