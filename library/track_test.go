package library

import (
	"testing"
	"time"

	"github.com/ronelliott/muzak/audio"
)

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		track Track
		want  string
	}{
		{
			name:  "artist and title",
			track: Track{Artist: "The Beatles", Title: "Hey Jude"},
			want:  "The Beatles - Hey Jude",
		},
		{
			name:  "title only",
			track: Track{Title: "Hey Jude"},
			want:  "Hey Jude",
		},
		{
			name:  "zip entry falls back to entry basename",
			track: Track{Path: "/archive.zip", ZipEntry: "albums/disc1/track01.wav", Format: audio.FormatWAV},
			want:  "track01.wav",
		},
		{
			name:  "plain file falls back to file basename",
			track: Track{Path: "/music/song.flac", Format: audio.FormatFLAC},
			want:  "song.flac",
		},
		{
			name:  "artist without title uses basename",
			track: Track{Artist: "Prince", Path: "/song.wav"},
			want:  "song.wav",
		},
		{
			name:  "zip entry preferred over path when no title",
			track: Track{Path: "/archive.zip", ZipEntry: "song.flac"},
			want:  "song.flac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Duration is irrelevant for display name; set it to avoid zero-value noise.
			tt.track.Duration = 3 * time.Minute
			got := tt.track.DisplayName()
			if got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
