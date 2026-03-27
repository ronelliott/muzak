package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ronelliott/muzak/audio"
)

const cacheVersion = 1

// diskCache is the top-level structure written to disk.
type diskCache struct {
	Version int                    `json:"version"`
	Entries map[string]*cacheEntry `json:"entries"` // key = absolute source path
}

// cacheEntry holds the fingerprint and cached metadata for one source file.
// For a ZIP archive the Tracks slice contains all audio entries within it.
type cacheEntry struct {
	ModTime int64        `json:"mod_time"` // UnixNano
	Size    int64        `json:"size"`
	Tracks  []cacheTrack `json:"tracks"`
}

// cacheTrack holds the serialisable subset of library.Track.
// Opener is omitted — it is always reconstructed from Path + ZipEntry.
type cacheTrack struct {
	Title    string       `json:"title,omitempty"`
	Artist   string       `json:"artist,omitempty"`
	Album    string       `json:"album,omitempty"`
	DurNs    int64        `json:"dur_ns"`
	ZipEntry string       `json:"zip_entry,omitempty"`
	Format   audio.Format `json:"format"`
}

// cacheFilePath returns the path to the cache file, creating the parent
// directory if necessary. If the MUZAK_CACHE_DIR environment variable is set
// it is used as the cache directory (useful in tests).
func cacheFilePath() (string, error) {
	dir := os.Getenv("MUZAK_CACHE_DIR")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("user cache dir: %w", err)
		}
		dir = filepath.Join(base, "muzak")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return filepath.Join(dir, "library.json"), nil
}

// loadDiskCache reads the cache from disk. It returns an empty, valid cache
// on any error (missing file, corrupt JSON, version mismatch) so the caller
// always gets a usable value.
func loadDiskCache() *diskCache {
	empty := &diskCache{Version: cacheVersion, Entries: make(map[string]*cacheEntry)}

	path, err := cacheFilePath()
	if err != nil {
		return empty
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "muzak: warning: read cache: %v\n", err)
		}
		return empty
	}

	var c diskCache
	if err := json.Unmarshal(data, &c); err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: parse cache: %v\n", err)
		return empty
	}
	if c.Version != cacheVersion {
		return empty
	}
	if c.Entries == nil {
		c.Entries = make(map[string]*cacheEntry)
	}
	return &c
}

// saveDiskCache writes the cache to disk atomically (temp file → rename).
// Errors are logged to stderr but not returned; a failed save just means the
// next startup will do a full scan.
func saveDiskCache(c *diskCache) {
	path, err := cacheFilePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: cache path: %v\n", err)
		return
	}

	data, err := json.Marshal(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: marshal cache: %v\n", err)
		return
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: write cache: %v\n", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		fmt.Fprintf(os.Stderr, "muzak: warning: rename cache: %v\n", err)
		os.Remove(tmp) //nolint:errcheck
	}
}

// buildCacheEntry converts freshly-scanned tracks into a cacheEntry using
// the provided FileInfo for the fingerprint.
func buildCacheEntry(info fs.FileInfo, tracks []*Track) *cacheEntry {
	ct := make([]cacheTrack, len(tracks))
	for i, t := range tracks {
		ct[i] = cacheTrack{
			Title:    t.Title,
			Artist:   t.Artist,
			Album:    t.Album,
			DurNs:    t.Duration.Nanoseconds(),
			ZipEntry: t.ZipEntry,
			Format:   t.Format,
		}
	}
	return &cacheEntry{
		ModTime: info.ModTime().UnixNano(),
		Size:    info.Size(),
		Tracks:  ct,
	}
}

// reconstructTracks rebuilds Track objects from a cache entry.
// Openers are always constructed fresh — function values are never serialised.
func reconstructTracks(sourcePath string, e *cacheEntry) []*Track {
	tracks := make([]*Track, 0, len(e.Tracks))
	for _, ct := range e.Tracks {
		t := &Track{
			Title:    ct.Title,
			Artist:   ct.Artist,
			Album:    ct.Album,
			Duration: time.Duration(ct.DurNs),
			Path:     sourcePath,
			ZipEntry: ct.ZipEntry,
			Format:   ct.Format,
		}
		if ct.ZipEntry == "" {
			path := sourcePath
			t.Opener = func() (io.ReadSeekCloser, error) { return os.Open(path) }
		} else {
			t.Opener = zipOpener(sourcePath, ct.ZipEntry)
		}
		tracks = append(tracks, t)
	}
	return tracks
}
