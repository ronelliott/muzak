package library

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ronelliott/muzak/audio"
)

// Track represents a single audio file discovered in the library.
type Track struct {
	Title    string
	Artist   string
	Album    string
	Duration time.Duration

	// Path is the filesystem path of the file (or ZIP archive).
	Path string
	// ZipEntry is the path of the entry inside the ZIP; empty for plain files.
	ZipEntry string
	// Format is the audio codec.
	Format audio.Format
	// Opener opens the track's audio data ready for decoding.
	Opener audio.Opener
}

// DisplayName returns a human-readable label for the track.
func (t *Track) DisplayName() string {
	if t.Artist != "" && t.Title != "" {
		return fmt.Sprintf("%s - %s", t.Artist, t.Title)
	}
	if t.Title != "" {
		return t.Title
	}
	name := t.ZipEntry
	if name == "" {
		name = t.Path
	}
	return filepath.Base(name)
}
