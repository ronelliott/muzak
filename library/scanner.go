package library

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhowden/tag"
	beepflac "github.com/gopxl/beep/flac"
	beepwav "github.com/gopxl/beep/wav"

	"github.com/ronelliott/muzak/audio"
)

// nopCloser wraps an io.ReadSeeker that requires no explicit closing.
type nopCloser struct{ io.ReadSeeker }

func (n nopCloser) Close() error { return nil }

// Scan recursively walks each directory (or SMB URL) for .flac/.wav files and
// .zip archives that contain them. Metadata is loaded from a per-file cache
// when the source file's mtime and size are unchanged; otherwise the file is
// scanned and the cache is updated.
func Scan(dirs []string) ([]*Track, error) {
	old := loadDiskCache()
	newCache := &diskCache{
		Version: cacheVersion,
		Entries: make(map[string]*cacheEntry, len(old.Entries)),
	}

	var tracks []*Track

	for _, dir := range dirs {
		if IsSMBPath(dir) {
			ts, err := scanSMB(dir, old, newCache)
			if err != nil {
				return nil, fmt.Errorf("scan SMB %s: %w", dir, err)
			}
			tracks = append(tracks, ts...)
			continue
		}

		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("resolve path %s: %w", dir, err)
		}
		dir = abs
		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".flac", ".wav":
				ts, entry := cachedOrScanFile(path, old)
				tracks = append(tracks, ts...)
				if entry != nil {
					newCache.Entries[path] = entry
				}
			case ".zip":
				ts, entry := cachedOrScanZip(path, old)
				tracks = append(tracks, ts...)
				if entry != nil {
					newCache.Entries[path] = entry
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", dir, err)
		}
	}

	saveDiskCache(newCache)
	return deduplicate(tracks), nil
}

// cachedOrScanFile returns tracks for a plain audio file, using the cache when
// the file's fingerprint matches.
func cachedOrScanFile(path string, c *diskCache) ([]*Track, *cacheEntry) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil
	}

	if entry, ok := c.Entries[path]; ok &&
		entry.ModTime == info.ModTime().UnixNano() &&
		entry.Size == info.Size() {
		return reconstructTracks(path, entry), entry
	}

	t, err := scanFile(path)
	if err != nil {
		return nil, nil
	}
	entry := buildCacheEntry(info, []*Track{t})
	return []*Track{t}, entry
}

// cachedOrScanZip returns tracks for a ZIP archive, using the cache when the
// archive's fingerprint matches.
func cachedOrScanZip(path string, c *diskCache) ([]*Track, *cacheEntry) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil
	}

	if entry, ok := c.Entries[path]; ok &&
		entry.ModTime == info.ModTime().UnixNano() &&
		entry.Size == info.Size() {
		return reconstructTracks(path, entry), entry
	}

	ts, err := scanZip(path)
	if err != nil || len(ts) == 0 {
		return nil, nil
	}
	entry := buildCacheEntry(info, ts)
	return ts, entry
}

// deduplicate removes tracks whose DisplayName (lowercased) has already been
// seen, keeping the first occurrence. Plain files are sorted before ZIP
// entries so they are preferred when a duplicate exists in both forms.
func deduplicate(tracks []*Track) []*Track {
	// Stable sort: plain files first, ZIP entries second.
	sort.SliceStable(tracks, func(i, j int) bool {
		iZip := tracks[i].ZipEntry != ""
		jZip := tracks[j].ZipEntry != ""
		return !iZip && jZip
	})

	seen := make(map[string]struct{}, len(tracks))
	out := tracks[:0]
	for _, t := range tracks {
		key := strings.ToLower(t.DisplayName())
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}

func formatFromName(name string) (audio.Format, bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".flac":
		return audio.FormatFLAC, true
	case ".wav":
		return audio.FormatWAV, true
	}
	return "", false
}

func scanFile(path string) (*Track, error) {
	format, ok := formatFromName(path)
	if !ok {
		return nil, fmt.Errorf("unsupported: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	t := &Track{
		Path:   path,
		Format: format,
		Opener: func() (io.ReadSeekCloser, error) { return os.Open(path) },
	}

	populateMetadata(t, f)
	f.Seek(0, io.SeekStart) //nolint:errcheck
	t.Duration = decodeDuration(f, format)
	return t, nil
}

func scanZip(zipPath string) ([]*Track, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var tracks []*Track
	for _, f := range r.File {
		format, ok := formatFromName(f.Name)
		if !ok {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		entryName := f.Name
		rs := bytes.NewReader(data)

		t := &Track{
			Path:     zipPath,
			ZipEntry: entryName,
			Format:   format,
			Opener:   zipOpener(zipPath, entryName),
		}

		populateMetadata(t, rs)
		rs.Seek(0, io.SeekStart) //nolint:errcheck
		t.Duration = decodeDuration(rs, format)
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// zipOpener returns an Opener that buffers the ZIP entry into memory at
// play time, which makes the resulting bytes.Reader seekable.
func zipOpener(zipPath, entryName string) audio.Opener {
	return func() (io.ReadSeekCloser, error) {
		r, err := zip.OpenReader(zipPath)
		if err != nil {
			return nil, fmt.Errorf("open zip %s: %w", zipPath, err)
		}
		defer r.Close()

		for _, f := range r.File {
			if f.Name != entryName {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open entry %s: %w", entryName, err)
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("read entry %s: %w", entryName, err)
			}
			return nopCloser{bytes.NewReader(data)}, nil
		}
		return nil, fmt.Errorf("entry %q not found in %s", entryName, zipPath)
	}
}

// populateMetadata reads tags from rs and writes them to t.
// Errors are silently ignored; the caller falls back to filename display.
func populateMetadata(t *Track, rs io.ReadSeeker) {
	m, err := tag.ReadFrom(rs)
	if err != nil {
		return
	}
	t.Title = m.Title()
	t.Artist = m.Artist()
	t.Album = m.Album()
}

// decodeDuration opens the audio stream just far enough to read total sample
// count, then converts to a time.Duration.
func decodeDuration(rs io.ReadSeeker, format audio.Format) time.Duration {
	switch format {
	case audio.FormatFLAC:
		s, f, err := beepflac.Decode(rs)
		if err != nil {
			return 0
		}
		d := f.SampleRate.D(s.Len())
		if c, ok := s.(io.Closer); ok {
			c.Close()
		}
		return d
	case audio.FormatWAV:
		s, f, err := beepwav.Decode(rs)
		if err != nil {
			return 0
		}
		d := f.SampleRate.D(s.Len())
		if c, ok := s.(io.Closer); ok {
			c.Close()
		}
		return d
	}
	return 0
}
