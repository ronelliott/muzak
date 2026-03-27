// Package audio defines the abstract playback backend interface.
package audio

import (
	"io"
	"time"
)

// Format is the audio container/codec format.
type Format string

const (
	FormatFLAC Format = "flac"
	FormatWAV  Format = "wav"
)

// Opener opens a track's audio data. It always returns a ReadSeekCloser so
// the caller can close the underlying resource when done.
type Opener func() (io.ReadSeekCloser, error)

// Backend is an abstract audio playback backend.
type Backend interface {
	// Init initializes the backend. Must be called once before Load.
	Init() error
	// Stop immediately halts all active playback and clears the output queue.
	Stop()
	// Load decodes a track and prepares it for playback (initially paused).
	Load(opener Opener, format Format) (Playback, error)
	// Close releases all backend resources.
	Close()
}

// Playback controls a single loaded track.
type Playback interface {
	// Play starts or resumes playback.
	Play()
	// Pause pauses playback.
	Pause()
	// IsPlaying reports whether the track is currently un-paused.
	IsPlaying() bool
	// Seek seeks to the given absolute position.
	Seek(d time.Duration) error
	// Duration returns the total duration of the track.
	Duration() time.Duration
	// Position returns the current playback position.
	Position() time.Duration
	// SetVolume sets the output volume. v must be in [0.0, 1.0].
	SetVolume(v float64)
	// Volume returns the current output volume.
	Volume() float64
	// Done returns a channel that is closed when the track ends naturally or
	// when Close is called.
	Done() <-chan struct{}
	// Close stops playback and releases resources.
	Close()
}
