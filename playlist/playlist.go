// Package playlist manages playback order, shuffle, repeat, and history.
package playlist

import (
	"math/rand"
	"time"

	"github.com/ronelliott/muzak/library"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

const maxHistory = 15

// HistoryEntry records a track that was played.
type HistoryEntry struct {
	Track      *library.Track
	TrackIndex int // stable index into Playlist.tracks
}

// Playlist manages a set of tracks with an ordered playback sequence.
type Playlist struct {
	tracks  []*library.Track
	order   []int // playback sequence; each element is an index into tracks
	cursor  int   // current position in order; -1 when nothing has started
	shuffle bool
	repeat  bool
	history []*HistoryEntry
}

// New creates a Playlist from the given tracks in their natural order.
func New(tracks []*library.Track) *Playlist {
	order := make([]int, len(tracks))
	for i := range order {
		order[i] = i
	}
	return &Playlist{tracks: tracks, order: order, cursor: -1}
}

// Tracks returns all tracks in their original (stable) order.
func (p *Playlist) Tracks() []*library.Track { return p.tracks }

// Len returns the total number of tracks.
func (p *Playlist) Len() int { return len(p.tracks) }

// Current returns the track at the current cursor, or nil.
func (p *Playlist) Current() *library.Track {
	if p.cursor < 0 || p.cursor >= len(p.order) {
		return nil
	}
	return p.tracks[p.order[p.cursor]]
}

// CurrentIndex returns the stable track index at the cursor, or -1.
func (p *Playlist) CurrentIndex() int {
	if p.cursor < 0 || p.cursor >= len(p.order) {
		return -1
	}
	return p.order[p.cursor]
}

// SetCursor moves the cursor to the given position in the playback order and
// records the track in history.
func (p *Playlist) SetCursor(orderPos int) {
	if orderPos < 0 || orderPos >= len(p.order) {
		return
	}
	p.cursor = orderPos
	p.recordHistory(p.tracks[p.order[orderPos]], p.order[orderPos])
}

// SetFirst moves the cursor to the first track in the current order.
func (p *Playlist) SetFirst() *library.Track {
	if len(p.order) == 0 {
		return nil
	}
	p.SetCursor(0)
	return p.Current()
}

// Prev moves to the previous track. When already at the first track and repeat
// is off, it stays on the first track. With repeat on it wraps to the last,
// reshuffling first when shuffle is also enabled (mirrors Next() behaviour).
func (p *Playlist) Prev() *library.Track {
	if len(p.order) == 0 {
		return nil
	}
	prev := p.cursor - 1
	if prev < 0 {
		if !p.repeat {
			prev = 0
		} else {
			if p.shuffle {
				p.applyShuffleAround(-1)
			}
			prev = len(p.order) - 1
		}
	}
	p.SetCursor(prev)
	return p.Current()
}

// Next advances to the next track. Returns nil if the playlist is exhausted
// and repeat is off.
func (p *Playlist) Next() *library.Track {
	if len(p.order) == 0 {
		return nil
	}
	next := p.cursor + 1
	if next >= len(p.order) {
		if !p.repeat {
			return nil
		}
		if p.shuffle {
			p.applyShuffleAround(-1)
		}
		next = 0
	}
	p.SetCursor(next)
	return p.Current()
}

// JumpToTrack finds the position of the given stable track index in the
// current order, sets the cursor there, and returns true on success.
func (p *Playlist) JumpToTrack(trackIndex int) bool {
	for i, ti := range p.order {
		if ti == trackIndex {
			p.SetCursor(i)
			return true
		}
	}
	return false
}

// Shuffle returns whether shuffle mode is active.
func (p *Playlist) Shuffle() bool { return p.shuffle }

// SetShuffle enables or disables shuffle mode. When enabling, the order is
// randomised but the current track stays at the cursor position.
func (p *Playlist) SetShuffle(on bool) {
	if p.shuffle == on {
		return
	}
	p.shuffle = on
	if on {
		p.applyShuffleAround(p.cursor)
	} else {
		p.restoreNaturalOrder()
	}
}

// Repeat returns whether repeat mode is active.
func (p *Playlist) Repeat() bool { return p.repeat }

// SetRepeat enables or disables repeat mode.
func (p *Playlist) SetRepeat(on bool) { p.repeat = on }

// History returns play history, oldest entry first, newest entry last.
// Maximum length is maxHistory (15).
func (p *Playlist) History() []*HistoryEntry { return p.history }

// ─── internal ────────────────────────────────────────────────────────────────

func (p *Playlist) recordHistory(t *library.Track, trackIndex int) {
	p.history = append(p.history, &HistoryEntry{Track: t, TrackIndex: trackIndex})
	if len(p.history) > maxHistory {
		p.history = p.history[len(p.history)-maxHistory:]
	}
}

// applyShuffleAround randomises p.order using Fisher-Yates and places the
// track currently at anchorOrderPos back into position anchorOrderPos so the
// currently-playing track does not change. Pass -1 to skip anchoring.
func (p *Playlist) applyShuffleAround(anchorOrderPos int) {
	var anchorTrack int = -1
	if anchorOrderPos >= 0 && anchorOrderPos < len(p.order) {
		anchorTrack = p.order[anchorOrderPos]
	}

	rng.Shuffle(len(p.order), func(i, j int) {
		p.order[i], p.order[j] = p.order[j], p.order[i]
	})

	if anchorTrack >= 0 {
		for i, ti := range p.order {
			if ti == anchorTrack {
				p.order[i], p.order[anchorOrderPos] = p.order[anchorOrderPos], p.order[i]
				break
			}
		}
		p.cursor = anchorOrderPos
	}
}

// restoreNaturalOrder resets the order to 0,1,2,… and updates the cursor to
// the natural index of the currently-playing track.
func (p *Playlist) restoreNaturalOrder() {
	currentTrack := -1
	if p.cursor >= 0 && p.cursor < len(p.order) {
		currentTrack = p.order[p.cursor]
	}
	for i := range p.order {
		p.order[i] = i
	}
	if currentTrack >= 0 {
		p.cursor = currentTrack // natural index == order position for identity permutation
	}
}
