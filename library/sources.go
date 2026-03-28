package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sourcesVersion = 1

// Sources holds the list of configured source directories persisted to
// $MUZAK_CONFIG_DIR/library.json.
type Sources struct {
	Version int      `json:"version"`
	Paths   []string `json:"sources"`
}

// LoadSources reads the sources list from disk, returning an empty list on
// any error so the caller always gets a usable value.
func LoadSources() *Sources {
	s := &Sources{Version: sourcesVersion}

	path, err := sourcesFilePath()
	if err != nil {
		return s
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "muzak: warning: read sources: %v\n", err)
		}
		return s
	}

	if err := json.Unmarshal(data, s); err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: parse sources: %v\n", err)
		return &Sources{Version: sourcesVersion}
	}

	if s.Version != sourcesVersion {
		fmt.Fprintf(os.Stderr, "muzak: warning: unsupported sources version %d (expected %d); ignoring\n", s.Version, sourcesVersion)
		return &Sources{Version: sourcesVersion}
	}

	return s
}

// Save writes the sources list to disk atomically.
func (s *Sources) Save() error {
	path, err := sourcesFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sources: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write sources: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("rename sources: %w", err)
	}
	return nil
}

// Add resolves path to an absolute path (or canonicalises SMB URLs) and
// appends it to the sources list if not already present. The caller is
// responsible for scanning and saving.
func (s *Sources) Add(path string) error {
	canonical := path
	if IsSMBPath(path) {
		canonical = canonicalizeSMBURL(path)
	} else {
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}
		canonical = abs
	}
	for _, p := range s.Paths {
		if p == canonical {
			return nil // already present
		}
	}
	s.Paths = append(s.Paths, canonical)
	return nil
}

// canonicalizeSMBURL normalises an SMB URL: lowercases scheme and host,
// strips the default port 445, and trims extraneous path slashes.
// Returns the input unchanged on parse error.
func canonicalizeSMBURL(rawURL string) string {
	cfg, err := parseSMBURL(rawURL)
	if err != nil {
		return rawURL
	}
	prefix := smbURLPrefix(cfg) // "smb://[user:pass@]host/share/"
	if cfg.subpath != "" {
		return prefix + cfg.subpath
	}
	return strings.TrimSuffix(prefix, "/")
}

// Remove removes path (resolved to absolute, or kept as-is for SMB URLs) from
// the sources list and prunes its entries from the disk cache. The caller is
// responsible for saving.
func (s *Sources) Remove(path string) error {
	abs := path
	if !IsSMBPath(path) {
		var err error
		abs, err = filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}
	}

	updated := s.Paths[:0]
	found := false
	for _, p := range s.Paths {
		if p == abs {
			found = true
			continue
		}
		updated = append(updated, p)
	}
	if !found {
		return fmt.Errorf("source not found: %s", abs)
	}
	s.Paths = updated

	pruneCache(abs)
	return nil
}

// Clear removes all sources and wipes the entire disk cache.
func (s *Sources) Clear() {
	s.Paths = nil
	clearCache()
}

// sourcesFilePath returns the path to the sources file, creating the parent
// directory if necessary.
func sourcesFilePath() (string, error) {
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
	return filepath.Join(dir, "library.json"), nil
}
