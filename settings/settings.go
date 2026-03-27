// Package settings handles loading and saving user preferences.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings holds all persisted user preferences.
type Settings struct {
	Volume float64 `json:"volume"`
}

const defaultVolume = 1.0

// Default returns a Settings with factory defaults.
func Default() Settings {
	return Settings{Volume: defaultVolume}
}

// Load reads settings from disk, returning defaults on any error.
func Load() Settings {
	s := Default()

	path, err := configFilePath()
	if err != nil {
		return s
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return s // missing file is expected on first run
	}

	if err := json.Unmarshal(data, &s); err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: parse settings: %v\n", err)
		return Default()
	}

	// Clamp to valid range in case the file was hand-edited.
	if s.Volume < 0 {
		s.Volume = 0
	}
	if s.Volume > 1 {
		s.Volume = 1
	}

	return s
}

// Save writes settings to disk atomically.
func Save(s Settings) {
	path, err := configFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: settings path: %v\n", err)
		return
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: marshal settings: %v\n", err)
		return
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: write settings: %v\n", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: rename settings: %v\n", err)
		os.Remove(tmp) //nolint:errcheck
	}
}

// configFilePath returns the path to the settings file, creating the parent
// directory if necessary. Respects the MUZAK_CONFIG_DIR environment variable.
func configFilePath() (string, error) {
	dir := os.Getenv("MUZAK_CONFIG_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("user config dir: %w", err)
		}
		dir = filepath.Join(base, "muzak")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return filepath.Join(dir, "settings.json"), nil
}
