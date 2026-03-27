package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ronelliott/muzak/audio"
)

// ─── buildCacheEntry / reconstructTracks round-trip ──────────────────────────

func TestBuildAndReconstructRoundTrip(t *testing.T) {
	original := []*Track{
		{
			Title:    "Hey Jude",
			Artist:   "The Beatles",
			Album:    "The Singles",
			Duration: 3*time.Minute + 12*time.Second,
			Path:     "/music/hey_jude.flac",
			Format:   audio.FormatFLAC,
		},
		{
			Title:    "Let It Be",
			Artist:   "The Beatles",
			Album:    "Let It Be",
			Duration: 4 * time.Minute,
			Path:     "/music/archive.zip",
			ZipEntry: "let_it_be.wav",
			Format:   audio.FormatWAV,
		},
	}

	info := fakeFileInfo{mtime: time.Now(), size: 12345}
	entry := buildCacheEntry(info, original)

	if len(entry.Tracks) != 2 {
		t.Fatalf("want 2 cached tracks, got %d", len(entry.Tracks))
	}
	if entry.ModTime != info.mtime.UnixNano() {
		t.Errorf("ModTime mismatch")
	}
	if entry.Size != info.size {
		t.Errorf("Size mismatch")
	}

	rebuilt := reconstructTracks("/music/archive.zip", entry)
	// Only the second track has a ZipEntry; pass the zip path as source.
	// Rebuild first track separately.
	rebuilt0 := reconstructTracks("/music/hey_jude.flac", &cacheEntry{Tracks: entry.Tracks[:1]})
	rebuilt1 := reconstructTracks("/music/archive.zip", &cacheEntry{Tracks: entry.Tracks[1:]})

	check := func(got, want *Track) {
		t.Helper()
		if got.Title != want.Title {
			t.Errorf("Title: got %q want %q", got.Title, want.Title)
		}
		if got.Artist != want.Artist {
			t.Errorf("Artist: got %q want %q", got.Artist, want.Artist)
		}
		if got.Album != want.Album {
			t.Errorf("Album: got %q want %q", got.Album, want.Album)
		}
		if got.Duration != want.Duration {
			t.Errorf("Duration: got %v want %v", got.Duration, want.Duration)
		}
		if got.Format != want.Format {
			t.Errorf("Format: got %q want %q", got.Format, want.Format)
		}
		if got.ZipEntry != want.ZipEntry {
			t.Errorf("ZipEntry: got %q want %q", got.ZipEntry, want.ZipEntry)
		}
		if got.Opener == nil {
			t.Error("Opener should not be nil")
		}
	}

	check(rebuilt0[0], original[0])
	check(rebuilt1[0], original[1])
	_ = rebuilt
}

// TestReconstructOpener_PlainFile verifies the reconstructed Opener can open
// a real file.
func TestReconstructOpener_PlainFile(t *testing.T) {
	dir := t.TempDir()
	path := writeWAVFile(t, dir, "song.wav")

	entry := &cacheEntry{
		Tracks: []cacheTrack{{Format: audio.FormatWAV}},
	}
	tracks := reconstructTracks(path, entry)
	if len(tracks) != 1 {
		t.Fatalf("want 1 track, got %d", len(tracks))
	}

	rc, err := tracks[0].Opener()
	if err != nil {
		t.Fatalf("Opener: %v", err)
	}
	rc.Close()
}

// TestReconstructOpener_ZipEntry verifies the reconstructed Opener can read
// a WAV entry from a ZIP archive.
func TestReconstructOpener_ZipEntry(t *testing.T) {
	dir := t.TempDir()
	zipPath := writeZipWithWAVs(t, dir, "album.zip", []string{"track.wav"})

	entry := &cacheEntry{
		Tracks: []cacheTrack{{ZipEntry: "track.wav", Format: audio.FormatWAV}},
	}
	tracks := reconstructTracks(zipPath, entry)
	if len(tracks) != 1 {
		t.Fatalf("want 1 track, got %d", len(tracks))
	}

	rc, err := tracks[0].Opener()
	if err != nil {
		t.Fatalf("Opener: %v", err)
	}
	rc.Close()
}

// ─── loadDiskCache ────────────────────────────────────────────────────────────

func TestLoadDiskCache_MissingFile(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	c := loadDiskCache()
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	if len(c.Entries) != 0 {
		t.Errorf("expected empty cache, got %d entries", len(c.Entries))
	}
}

func TestLoadDiskCache_CorruptJSON(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("MUZAK_CACHE_DIR", cacheDir)
	if err := os.WriteFile(filepath.Join(cacheDir, "library.json"), []byte("not json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := loadDiskCache()
	if len(c.Entries) != 0 {
		t.Error("corrupt cache should return empty result")
	}
}

func TestLoadDiskCache_VersionMismatch(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("MUZAK_CACHE_DIR", cacheDir)

	stale := diskCache{Version: 999, Entries: map[string]*cacheEntry{
		"/some/file.wav": {ModTime: 1, Size: 1},
	}}
	data, _ := json.Marshal(stale)
	os.WriteFile(filepath.Join(cacheDir, "library.json"), data, 0o644) //nolint:errcheck

	c := loadDiskCache()
	if len(c.Entries) != 0 {
		t.Error("version-mismatched cache should return empty result")
	}
}

// ─── saveDiskCache / loadDiskCache round-trip ─────────────────────────────────

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())

	original := &diskCache{
		Version: cacheVersion,
		Entries: map[string]*cacheEntry{
			"/music/song.flac": {
				ModTime: 1234567890,
				Size:    99999,
				Tracks: []cacheTrack{
					{Title: "Song", Artist: "Artist", DurNs: int64(3 * time.Minute), Format: audio.FormatFLAC},
				},
			},
		},
	}

	saveDiskCache(original)
	loaded := loadDiskCache()

	if loaded.Version != cacheVersion {
		t.Errorf("Version: got %d want %d", loaded.Version, cacheVersion)
	}
	e, ok := loaded.Entries["/music/song.flac"]
	if !ok {
		t.Fatal("entry not found after round-trip")
	}
	if e.ModTime != 1234567890 {
		t.Errorf("ModTime: got %d want 1234567890", e.ModTime)
	}
	if len(e.Tracks) != 1 || e.Tracks[0].Title != "Song" {
		t.Errorf("Tracks not preserved: %+v", e.Tracks)
	}
}

// ─── Scan cache integration ───────────────────────────────────────────────────

func TestScan_WarmCacheProducesSameResults(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	writeWAVFile(t, dir, "song.wav")

	first, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("first Scan: %v", err)
	}
	second, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("second Scan: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("track count differs: first=%d second=%d", len(first), len(second))
	}
	if first[0].DisplayName() != second[0].DisplayName() {
		t.Errorf("DisplayName differs: %q vs %q", first[0].DisplayName(), second[0].DisplayName())
	}
	if first[0].Duration != second[0].Duration {
		t.Errorf("Duration differs: %v vs %v", first[0].Duration, second[0].Duration)
	}
}

func TestScan_CacheInvalidatedOnMtimeChange(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("MUZAK_CACHE_DIR", cacheDir)
	dir := t.TempDir()
	path := writeWAVFile(t, dir, "song.wav")

	// First scan: populate cache.
	if _, err := Scan([]string{dir}); err != nil {
		t.Fatalf("first Scan: %v", err)
	}

	// Advance the file's mtime by 1 second.
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// Second scan: cache entry fingerprint should miss; file re-scanned.
	if _, err := Scan([]string{dir}); err != nil {
		t.Fatalf("second Scan: %v", err)
	}

	// The new cache entry must carry the updated mtime.
	c := loadDiskCache()
	entry, ok := c.Entries[path]
	if !ok {
		t.Fatal("expected cache entry for file")
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ModTime != stat.ModTime().UnixNano() {
		t.Errorf("cache mtime not updated: got %d want %d", entry.ModTime, stat.ModTime().UnixNano())
	}
}

func TestScan_OrphanedEntriesDropped(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("MUZAK_CACHE_DIR", cacheDir)
	dir := t.TempDir()
	path := writeWAVFile(t, dir, "song.wav")

	// First scan: cache has one entry.
	if _, err := Scan([]string{dir}); err != nil {
		t.Fatalf("first Scan: %v", err)
	}

	// Remove the file.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	// Second scan: no files found; orphaned entry should be gone.
	if _, err := Scan([]string{dir}); err != nil {
		t.Fatalf("second Scan: %v", err)
	}

	c := loadDiskCache()
	if _, ok := c.Entries[path]; ok {
		t.Error("orphaned cache entry should have been removed")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// fakeFileInfo implements fs.FileInfo for testing buildCacheEntry.
type fakeFileInfo struct {
	mtime time.Time
	size  int64
}

func (f fakeFileInfo) Name() string      { return "fake" }
func (f fakeFileInfo) Size() int64       { return f.size }
func (f fakeFileInfo) Mode() os.FileMode { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return f.mtime }
func (f fakeFileInfo) IsDir() bool       { return false }
func (f fakeFileInfo) Sys() any          { return nil }
