package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/ronelliott/muzak/playlist"
)

// ─── styles ──────────────────────────────────────────────────────────────────

var (
	styleStatus     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226"))
	styleNowPlaying = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	stylePaused     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	styleSelected   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	styleControls   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleDim        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleHeader     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleFlag       = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
)

// ─── View ─────────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}

	var sb strings.Builder

	if m.mode == modeLibrary {
		m.renderLibrary(&sb)
	} else {
		m.renderHistory(&sb)
	}

	sb.WriteString(m.renderControls())
	sb.WriteByte('\n')
	sb.WriteString(m.renderNowPlaying())

	return sb.String()
}

// ─── history view ─────────────────────────────────────────────────────────────

func (m *Model) renderHistory(sb *strings.Builder) {
	history := m.playlist.History()

	const maxH = 15
	if len(history) > maxH {
		history = history[len(history)-maxH:]
	}

	histAreaHeight := m.height - 2
	if histAreaHeight < 0 {
		histAreaHeight = 0
	}

	// Float entries to the bottom; pad the top with blank lines.
	pad := histAreaHeight - len(history)
	for i := 0; i < pad; i++ {
		sb.WriteByte('\n')
	}

	for i, entry := range history {
		sb.WriteString(m.renderHistoryRow(entry, i == m.historyCursor))
		sb.WriteByte('\n')
	}
}

func (m *Model) renderHistoryRow(e *playlist.HistoryEntry, selected bool) string {
	name := truncate(e.Track.DisplayName(), m.width-2)
	if selected {
		return styleSelected.Render("► ") + styleSelected.Render(name)
	}
	return "  " + styleDim.Render(name)
}

// ─── library overlay ─────────────────────────────────────────────────────────

func (m *Model) renderLibrary(sb *strings.Builder) {
	searchDisplay := m.libSearch + "_"
	hint := "  ↑↓=nav  ↵=play  Esc=close"
	// Truncate the search display so the header fits within the terminal width.
	// "Library" (7) + "  /" (3) + hint (len) = fixed overhead; leave the rest for the query.
	fixedLen := 7 + 3 + len([]rune(hint))
	queryMax := m.width - fixedLen
	if queryMax < 1 {
		queryMax = 1
	}
	header := styleHeader.Render("Library") +
		styleNowPlaying.Render("  /"+truncate(searchDisplay, queryMax)) +
		styleControls.Render(hint)
	sb.WriteString(header)
	sb.WriteByte('\n')

	visLines := m.libraryVisibleLines()
	tracks := m.libFiltered

	end := m.libOffset + visLines
	if end > len(tracks) {
		end = len(tracks)
	}
	visible := tracks[m.libOffset:end]

	for i, t := range visible {
		absIdx := m.libOffset + i
		selected := absIdx == m.libCursor
		name := truncate(t.DisplayName(), m.width-2)
		if selected {
			sb.WriteString(styleSelected.Render("► ") + styleSelected.Render(name))
		} else {
			sb.WriteString("  " + name)
		}
		sb.WriteByte('\n')
	}

	// Pad so controls/now-playing remain anchored.
	rendered := len(visible) + 1 // +1 for header
	pad := (m.height - 2) - rendered
	for i := 0; i < pad; i++ {
		sb.WriteByte('\n')
	}
}

// ─── controls & now-playing ───────────────────────────────────────────────────

func (m *Model) renderControls() string {
	line := helpLine
	if m.mode == modeLibrary {
		line = helpLineLibrary
	}
	return styleControls.Render(truncate(line, m.width))
}

func (m *Model) renderNowPlaying() string {
	track := m.playlist.Current()
	if track == nil {
		return styleControls.Render("No tracks loaded")
	}

	vol := int(m.volume * 100)
	progress := fmt.Sprintf("[%s / %s]", fmtDur(m.position), fmtDur(m.duration))

	var flags string
	if m.playlist.Shuffle() {
		flags += styleFlag.Render(" [S]")
	}
	if m.playlist.Repeat() {
		flags += styleFlag.Render(" [R]")
	}

	// Build meta separately so pre-rendered flag colors aren't clobbered.
	meta := styleControls.Render(fmt.Sprintf("  %s  Vol:%d%%", progress, vol)) + flags

	name := truncate(track.DisplayName(), m.width/2)
	switch {
	case m.current != nil && m.current.IsPlaying():
		return styleStatus.Render("▶") + " " + styleNowPlaying.Render(name) + meta
	case m.current == nil:
		return styleStatus.Render("⏹") + " " + stylePaused.Render(name) + meta // end of playlist
	default:
		return styleStatus.Render("⏸") + " " + stylePaused.Render(name) + meta
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// fmtDur formats a duration as m:ss.
func fmtDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

// truncate shortens s to at most n runes (adds "…" if cut).
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(runes[:n-1]) + "…"
}
