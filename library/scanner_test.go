package library

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ronelliott/muzak/audio"
)

// ─── deduplicate ─────────────────────────────────────────────────────────────

func TestDeduplicate_NoDuplicates(t *testing.T) {
	tracks := []*Track{
		{Title: "Alpha", Artist: "A", Format: audio.FormatWAV},
		{Title: "Beta", Artist: "B", Format: audio.FormatWAV},
		{Title: "Gamma", Artist: "C", Format: audio.FormatWAV},
	}
	got := deduplicate(tracks)
	if len(got) != 3 {
		t.Fatalf("want 3 tracks, got %d", len(got))
	}
}

func TestDeduplicate_RemovesCaseInsensitiveDuplicate(t *testing.T) {
	tracks := []*Track{
		{Title: "Hey Jude", Artist: "The Beatles", Format: audio.FormatWAV},
		{Title: "hey jude", Artist: "the beatles", Format: audio.FormatWAV},
	}
	got := deduplicate(tracks)
	if len(got) != 1 {
		t.Fatalf("want 1 track, got %d", len(got))
	}
}

func TestDeduplicate_PrefersPlainFileOverZip(t *testing.T) {
	plain := &Track{
		Title:  "Song",
		Artist: "Artist",
		Path:   "/music/song.wav",
		Format: audio.FormatWAV,
	}
	zipped := &Track{
		Title:    "Song",
		Artist:   "Artist",
		Path:     "/archive.zip",
		ZipEntry: "song.wav",
		Format:   audio.FormatWAV,
	}
	// Pass zip first to confirm ordering isn't input-order dependent.
	got := deduplicate([]*Track{zipped, plain})
	if len(got) != 1 {
		t.Fatalf("want 1 track, got %d", len(got))
	}
	if got[0].ZipEntry != "" {
		t.Errorf("expected plain file to win, got zip entry %q", got[0].ZipEntry)
	}
}

func TestDeduplicate_TwoZips_KeepsFirst(t *testing.T) {
	a := &Track{Title: "Song", Artist: "Artist", Path: "/a.zip", ZipEntry: "song.wav", Format: audio.FormatWAV}
	b := &Track{Title: "Song", Artist: "Artist", Path: "/b.zip", ZipEntry: "song.wav", Format: audio.FormatWAV}
	got := deduplicate([]*Track{a, b})
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}

func TestDeduplicate_PreservesDistinctTracks(t *testing.T) {
	tracks := []*Track{
		{Title: "One", Artist: "A", Format: audio.FormatWAV},
		{Title: "Two", Artist: "A", Format: audio.FormatWAV},
		{Title: "One", Artist: "A", Format: audio.FormatWAV}, // duplicate
	}
	got := deduplicate(tracks)
	if len(got) != 2 {
		t.Fatalf("want 2 tracks, got %d", len(got))
	}
}

// ─── Scan end-to-end ─────────────────────────────────────────────────────────

func scanDir(t *testing.T) (string, func()) {
	t.Helper()
	cacheDir := t.TempDir()
	t.Setenv("MUZAK_CACHE_DIR", cacheDir)
	dir := t.TempDir()
	return dir, func() {}
}

func TestScan_FindsWAVFile(t *testing.T) {
	dir, _ := scanDir(t)
	writeWAVFile(t, dir, "song.wav")

	tracks, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("want 1 track, got %d", len(tracks))
	}
	if tracks[0].Format != audio.FormatWAV {
		t.Errorf("want FormatWAV, got %q", tracks[0].Format)
	}
}

func TestScan_RecursiveWalk(t *testing.T) {
	dir, _ := scanDir(t)
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeWAVFile(t, dir, "a.wav")
	writeWAVFile(t, sub, "b.wav")

	tracks, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("want 2 tracks, got %d", len(tracks))
	}
}

func TestScan_IgnoresHiddenFiles(t *testing.T) {
	dir, _ := scanDir(t)
	writeWAVFile(t, dir, "visible.wav")
	writeWAVFile(t, dir, ".hidden.wav")

	tracks, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("want 1 track, got %d", len(tracks))
	}
	if tracks[0].DisplayName() == ".hidden.wav" {
		t.Error("hidden file should have been excluded")
	}
}

func TestScan_IgnoresHiddenDirectory(t *testing.T) {
	dir, _ := scanDir(t)
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.Mkdir(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeWAVFile(t, hiddenDir, "song.wav")

	tracks, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 0 {
		t.Fatalf("want 0 tracks, got %d", len(tracks))
	}
}

func TestScan_ZipEntries(t *testing.T) {
	dir, _ := scanDir(t)
	writeZipWithWAVs(t, dir, "album.zip", []string{"track01.wav", "track02.wav"})

	tracks, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("want 2 tracks, got %d", len(tracks))
	}
	for _, tr := range tracks {
		if tr.ZipEntry == "" {
			t.Errorf("track %q should have a ZipEntry", tr.DisplayName())
		}
	}
}

func TestScan_DeduplicatesPlainAndZip(t *testing.T) {
	dir, _ := scanDir(t)
	// Plain file — should win dedup over the zip entry with the same title.
	// Both have no metadata so DisplayName falls back to the base filename.
	writeWAVFile(t, dir, "song.wav")
	writeZipWithWAVs(t, dir, "album.zip", []string{"song.wav"})

	tracks, err := Scan([]string{dir})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("want 1 track after dedup, got %d", len(tracks))
	}
	if tracks[0].ZipEntry != "" {
		t.Errorf("plain file should have won dedup, got zip entry %q", tracks[0].ZipEntry)
	}
}

func TestScan_MultipleDirs(t *testing.T) {
	t.Setenv("MUZAK_CACHE_DIR", t.TempDir())
	dirA := t.TempDir()
	dirB := t.TempDir()
	writeWAVFile(t, dirA, "a.wav")
	writeWAVFile(t, dirB, "b.wav")

	tracks, err := Scan([]string{dirA, dirB})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("want 2 tracks, got %d", len(tracks))
	}
}
