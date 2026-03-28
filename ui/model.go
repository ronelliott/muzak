// Package ui implements the BubbleTea terminal interface for muzak.
package ui

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ronelliott/muzak/audio"
	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/muzak/playlist"
	"github.com/ronelliott/muzak/settings"
)

// ─── messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time

// trackDoneMsg is sent when a track finishes. The id field lets us discard
// stale messages after the user has already skipped to a new track.
type trackDoneMsg struct{ id uint64 }

type uiMode int

const (
	modePlayer  uiMode = iota
	modeLibrary        // library overlay active
)

const (
	volumeStep = 0.05
	seekStep   = 10 * time.Second
)

// Model is the root BubbleTea model.
type Model struct {
	backend  audio.Backend
	playlist *playlist.Playlist

	// Sorted track slice used by the library overlay.
	libTracks    []*library.Track
	libFiltered  []*library.Track // subset of libTracks matching libSearch
	libSearch    string           // current filter query

	// Current playback state.
	current  audio.Playback
	trackID  uint64
	position time.Duration
	duration time.Duration
	volume   float64

	// UI state.
	mode          uiMode
	historyCursor int // index into playlist.History(); -1 = no selection
	libCursor     int // index into libFiltered
	libOffset     int // first visible row in library list

	// Terminal size.
	width  int
	height int
}

// NewModel creates a model wired to the given backend and playlist.
func NewModel(backend audio.Backend, pl *playlist.Playlist) *Model {
	// Build a display-sorted copy of all tracks for the library overlay.
	lib := make([]*library.Track, len(pl.Tracks()))
	copy(lib, pl.Tracks())
	sort.Slice(lib, func(i, j int) bool {
		a, b := lib[i], lib[j]
		if a.Artist != b.Artist {
			return a.Artist < b.Artist
		}
		if a.Album != b.Album {
			return a.Album < b.Album
		}
		return a.DisplayName() < b.DisplayName()
	})

	s := settings.Load()
	m := &Model{
		backend:       backend,
		playlist:      pl,
		libTracks:     lib,
		volume:        s.Volume,
		historyCursor: -1,
	}
	m.libFiltered = m.libTracks
	return m
}

// ─── BubbleTea interface ──────────────────────────────────────────────────────

func (m *Model) Init() tea.Cmd {
	if m.playlist.Len() == 0 {
		return tick()
	}
	return tea.Batch(tick(), func() tea.Msg { return startMsg{} })
}

type startMsg struct{}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case startMsg:
		m.playlist.SetFirst()
		return m, m.cmdLoadAndPlay()

	case tickMsg:
		if m.current != nil {
			m.position = m.current.Position()
			m.duration = m.current.Duration()
		}
		return m, tick()

	case trackDoneMsg:
		if msg.id != m.trackID {
			return m, nil // stale
		}
		next := m.playlist.Next()
		if next == nil {
			if m.current != nil {
				m.current.Close()
				m.current = nil
			}
			return m, nil
		}
		return m, m.cmdLoadAndPlay()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// ─── key handling ────────────────────────────────────────────────────────────

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Keys that work in any mode.
	switch key {
	case "ctrl+c", keyQuit:
		if m.current != nil {
			m.backend.Stop()
			m.current.Close()
		}
		return m, tea.Quit

	case keyPause:
		if m.current != nil {
			if m.current.IsPlaying() {
				m.current.Pause()
			} else {
				m.current.Play()
			}
		}
		return m, nil

	case keyVolUp:
		m.volume = clamp(m.volume+volumeStep, 0, 1)
		if m.current != nil {
			m.current.SetVolume(m.volume)
		}
		settings.Save(settings.Settings{Volume: m.volume})
		return m, nil

	case keyVolDown:
		m.volume = clamp(m.volume-volumeStep, 0, 1)
		if m.current != nil {
			m.current.SetVolume(m.volume)
		}
		settings.Save(settings.Settings{Volume: m.volume})
		return m, nil

	case keyShuffle:
		m.playlist.SetShuffle(!m.playlist.Shuffle())
		return m, nil

	case keyRepeat:
		m.playlist.SetRepeat(!m.playlist.Repeat())
		return m, nil
	}

	if m.mode == modeLibrary {
		return m.handleLibraryKey(msg)
	}
	return m.handlePlayerKey(key)
}

func (m *Model) handlePlayerKey(key string) (tea.Model, tea.Cmd) {
	history := m.playlist.History()

	switch key {
	case keyPrev:
		m.playlist.Prev()
		return m, m.cmdLoadAndPlay()

	case keyNext:
		next := m.playlist.Next()
		if next == nil {
			return m, nil
		}
		return m, m.cmdLoadAndPlay()

	case keyRewind:
		if m.current != nil {
			target := m.position - seekStep
			if target < 0 {
				target = 0
			}
			m.current.Seek(target) //nolint:errcheck
		}

	case keyForward:
		if m.current != nil {
			m.current.Seek(m.position + seekStep) //nolint:errcheck
		}

	case "up":
		if m.historyCursor < 0 {
			m.historyCursor = len(history) - 1
		} else if m.historyCursor > 0 {
			m.historyCursor--
		}

	case "down":
		if m.historyCursor >= 0 && m.historyCursor < len(history)-1 {
			m.historyCursor++
		} else if m.historyCursor == len(history)-1 {
			m.historyCursor = -1 // deselect at bottom
		}

	case "enter":
		if m.historyCursor >= 0 && m.historyCursor < len(history) {
			entry := history[m.historyCursor]
			m.playlist.JumpToTrack(entry.TrackIndex)
			m.historyCursor = -1
			return m, m.cmdLoadAndPlay()
		}

	case keyLibrary:
		m.mode = modeLibrary
	}

	return m, nil
}

func (m *Model) handleLibraryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	n := len(m.libFiltered)
	visLines := m.libraryVisibleLines()

	switch {
	case key == "esc" || key == keyLibrary:
		m.mode = modePlayer

	case key == "up":
		if m.libCursor > 0 {
			m.libCursor--
			if m.libCursor < m.libOffset {
				m.libOffset = m.libCursor
			}
		}

	case key == "down":
		if m.libCursor < n-1 {
			m.libCursor++
			if m.libCursor >= m.libOffset+visLines {
				m.libOffset = m.libCursor - visLines + 1
			}
		}

	case key == "enter":
		if m.libCursor < n {
			t := m.libFiltered[m.libCursor]
			for idx, pt := range m.playlist.Tracks() {
				if pt == t {
					m.playlist.JumpToTrack(idx)
					break
				}
			}
			m.mode = modePlayer
			m.historyCursor = -1
			return m, m.cmdLoadAndPlay()
		}

	case key == "backspace" || key == "ctrl+h":
		if r := []rune(m.libSearch); len(r) > 0 {
			m.libSearch = string(r[:len(r)-1])
			m.updateLibFilter()
		}

	case msg.Type == tea.KeyRunes:
		m.libSearch += string(msg.Runes)
		m.updateLibFilter()
	}

	return m, nil
}

// updateLibFilter recomputes libFiltered from libTracks using libSearch and
// resets the cursor to the top of the results.
func (m *Model) updateLibFilter() {
	query := strings.ToLower(m.libSearch)
	if query == "" {
		m.libFiltered = m.libTracks
	} else {
		filtered := make([]*library.Track, 0, len(m.libTracks))
		for _, t := range m.libTracks {
			if strings.Contains(strings.ToLower(t.DisplayName()), query) {
				filtered = append(filtered, t)
			}
		}
		m.libFiltered = filtered
	}
	m.libCursor = 0
	m.libOffset = 0
}

// ─── playback helpers ─────────────────────────────────────────────────────────

// cmdLoadAndPlay loads the current playlist track and starts it playing.
// It returns a command that waits for the track to finish.
func (m *Model) cmdLoadAndPlay() tea.Cmd {
	track := m.playlist.Current()
	if track == nil {
		return nil
	}

	if m.current != nil {
		m.backend.Stop()
		m.current.Close()
	}

	pb, err := m.backend.Load(track.Opener, track.Format)
	if err != nil {
		m.current = nil
		m.position = 0
		m.duration = 0
		// Skip to the next track rather than stalling on an unplayable file.
		next := m.playlist.Next()
		if next == nil {
			return nil
		}
		return m.cmdLoadAndPlay()
	}

	m.trackID++
	id := m.trackID
	m.current = pb
	m.position = 0
	m.duration = pb.Duration()
	pb.SetVolume(m.volume)
	pb.Play()

	m.historyCursor = -1 // reset so the history list shows the newest entry at bottom

	return func() tea.Msg {
		<-pb.Done()
		return trackDoneMsg{id: id}
	}
}

// ─── utilities ───────────────────────────────────────────────────────────────

func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// libraryVisibleLines returns how many track rows fit in the library overlay.
func (m *Model) libraryVisibleLines() int {
	lines := m.height - 3 // header + controls + now-playing
	if lines < 1 {
		return 1
	}
	return lines
}
