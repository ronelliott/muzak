package library

import (
	"path/filepath"
	"testing"
)

func TestLoadSources_MissingFile(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())
	s := LoadSources()
	if s == nil {
		t.Fatal("expected non-nil Sources")
	}
	if len(s.Paths) != 0 {
		t.Errorf("expected empty sources, got %v", s.Paths)
	}
}

func TestSourcesSaveAndLoad(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())

	s := LoadSources()
	s.Paths = []string{"/music/jazz", "/music/rock"}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded := LoadSources()
	if len(loaded.Paths) != 2 {
		t.Fatalf("want 2 sources, got %d", len(loaded.Paths))
	}
	if loaded.Paths[0] != "/music/jazz" || loaded.Paths[1] != "/music/rock" {
		t.Errorf("unexpected paths: %v", loaded.Paths)
	}
}

func TestSourcesAdd(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()

	s := LoadSources()
	if err := s.Add(dir); err != nil {
		t.Fatalf("Add: %v", err)
	}

	abs, _ := filepath.Abs(dir)
	if len(s.Paths) != 1 || s.Paths[0] != abs {
		t.Errorf("want [%s], got %v", abs, s.Paths)
	}
}

func TestSourcesAdd_Deduplication(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()

	s := LoadSources()
	s.Add(dir) //nolint:errcheck
	s.Add(dir) //nolint:errcheck

	if len(s.Paths) != 1 {
		t.Errorf("duplicate add should not grow list: got %v", s.Paths)
	}
}

func TestSourcesAdd_ResolvesToAbsolute(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()

	s := LoadSources()
	if err := s.Add(dir); err != nil {
		t.Fatal(err)
	}

	if !filepath.IsAbs(s.Paths[0]) {
		t.Errorf("expected absolute path, got %q", s.Paths[0])
	}
}

func TestSourcesRemove(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	dir := t.TempDir()

	s := LoadSources()
	s.Add(dir) //nolint:errcheck

	if err := s.Remove(dir); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(s.Paths) != 0 {
		t.Errorf("expected empty sources after remove, got %v", s.Paths)
	}
}

func TestSourcesRemove_NotFound(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())

	s := LoadSources()
	if err := s.Remove("/nonexistent"); err == nil {
		t.Error("expected error when removing non-existent source")
	}
}

func TestSourcesClear(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())

	s := LoadSources()
	s.Paths = []string{"/music/jazz", "/music/rock"}
	s.Clear()

	if len(s.Paths) != 0 {
		t.Errorf("expected empty paths after Clear, got %v", s.Paths)
	}
}
