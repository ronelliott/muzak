package library

import (
	"path/filepath"
	"testing"
)

func TestLoadFromCache_Empty(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	tracks := LoadFromCache([]string{"/nonexistent"})
	if len(tracks) != 0 {
		t.Errorf("expected empty result for uncached path, got %d", len(tracks))
	}
}

func TestLoadFromCache_ReturnsCachedTracks(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	writeWAVFile(t, dir, "song.wav")

	if _, err := Scan([]string{dir}); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	tracks := LoadFromCache([]string{dir})
	if len(tracks) == 0 {
		t.Fatal("expected tracks from cache, got none")
	}
}

func TestPruneCache(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	path := writeWAVFile(t, dir, "song.wav")

	if _, err := Scan([]string{dir}); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	abs, _ := filepath.Abs(path)
	pruneCache(dir)

	c := loadDiskCache()
	if _, ok := c.Entries[abs]; ok {
		t.Error("expected cache entry to be pruned")
	}
}

func TestClearCache(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	writeWAVFile(t, dir, "song.wav")

	if _, err := Scan([]string{dir}); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	clearCache()

	c := loadDiskCache()
	if len(c.Entries) != 0 {
		t.Errorf("expected empty cache after clear, got %d entries", len(c.Entries))
	}
}
