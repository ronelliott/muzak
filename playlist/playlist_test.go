package playlist

import (
	"testing"

	"github.com/ronelliott/muzak/library"
)

// makeTracks builds n library.Track values each with Title "Track" and Artist "A", "B", …
func makeTracks(n int) []*library.Track {
	tracks := make([]*library.Track, n)
	for i := range tracks {
		tracks[i] = &library.Track{Title: "Track", Artist: string(rune('A' + i))}
	}
	return tracks
}

// orderOf returns a copy of the playlist's internal order slice for inspection.
func orderOf(p *Playlist) []int {
	o := make([]int, len(p.order))
	copy(o, p.order)
	return o
}

// ─── New ─────────────────────────────────────────────────────────────────────

func TestNew_NaturalOrder(t *testing.T) {
	tracks := makeTracks(3)
	p := New(tracks)

	for i, v := range p.order {
		if v != i {
			t.Errorf("order[%d] = %d, want %d", i, v, i)
		}
	}
}

func TestNew_CursorIsMinusOne(t *testing.T) {
	p := New(makeTracks(3))
	if p.cursor != -1 {
		t.Errorf("initial cursor = %d, want -1", p.cursor)
	}
}

func TestNew_Empty(t *testing.T) {
	p := New(nil)
	if p.Current() != nil {
		t.Error("Current() on empty playlist should be nil")
	}
	if p.SetFirst() != nil {
		t.Error("SetFirst() on empty playlist should be nil")
	}
}

// ─── SetFirst / Current ───────────────────────────────────────────────────────

func TestSetFirst_ReturnFirstTrack(t *testing.T) {
	tracks := makeTracks(3)
	p := New(tracks)
	got := p.SetFirst()
	if got != tracks[0] {
		t.Errorf("SetFirst() returned wrong track")
	}
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0", p.cursor)
	}
}

func TestSetFirst_RecordsHistory(t *testing.T) {
	p := New(makeTracks(3))
	p.SetFirst()
	if len(p.History()) != 1 {
		t.Errorf("want 1 history entry, got %d", len(p.History()))
	}
}

// ─── Next ─────────────────────────────────────────────────────────────────────

func TestNext_AdvancesCursor(t *testing.T) {
	tracks := makeTracks(3)
	p := New(tracks)
	p.SetFirst()

	got := p.Next()
	if got != tracks[1] {
		t.Errorf("Next() returned wrong track")
	}
	if p.cursor != 1 {
		t.Errorf("cursor = %d, want 1", p.cursor)
	}
}

func TestNext_ReturnsNilAtEndWithoutRepeat(t *testing.T) {
	p := New(makeTracks(2))
	p.SetFirst()
	p.Next() // advance to last

	got := p.Next() // past end
	if got != nil {
		t.Errorf("Next() at end without repeat should be nil, got %v", got)
	}
}

func TestNext_WrapsAtEndWithRepeat(t *testing.T) {
	tracks := makeTracks(3)
	p := New(tracks)
	p.SetRepeat(true)
	p.SetFirst()
	p.Next()
	p.Next() // now at last (index 2)

	got := p.Next() // should wrap to index 0
	if got != tracks[0] {
		t.Errorf("Next() with repeat should wrap to first track")
	}
	if p.cursor != 0 {
		t.Errorf("cursor after wrap = %d, want 0", p.cursor)
	}
}

// ─── Prev ─────────────────────────────────────────────────────────────────────

func TestPrev_MovesBackFromMiddle(t *testing.T) {
	tracks := makeTracks(3)
	p := New(tracks)
	p.SetFirst()
	p.Next() // cursor = 1

	got := p.Prev()
	if got != tracks[0] {
		t.Errorf("Prev() returned wrong track")
	}
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0", p.cursor)
	}
}

func TestPrev_ClampsAtFirstTrackWithoutRepeat(t *testing.T) {
	tracks := makeTracks(3)
	p := New(tracks)
	p.SetFirst() // cursor = 0

	got := p.Prev()
	if got != tracks[0] {
		t.Errorf("Prev() at first track without repeat should return first track")
	}
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0", p.cursor)
	}
}

func TestPrev_WrapsToLastWithRepeat(t *testing.T) {
	tracks := makeTracks(3)
	p := New(tracks)
	p.SetRepeat(true)
	p.SetFirst() // cursor = 0

	got := p.Prev()
	if got != tracks[2] {
		t.Errorf("Prev() with repeat should wrap to last track")
	}
	if p.cursor != 2 {
		t.Errorf("cursor after wrap = %d, want 2", p.cursor)
	}
}

func TestPrev_ShuffleRepeat_ReshufflesOnWrap(t *testing.T) {
	p := New(makeTracks(8))
	p.SetRepeat(true)
	p.SetShuffle(true)
	p.SetFirst() // cursor = 0
	orderBefore := orderOf(p)

	p.Prev() // should wrap and reshuffle

	orderAfter := orderOf(p)
	same := true
	for i := range orderBefore {
		if orderBefore[i] != orderAfter[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Prev() with shuffle+repeat should reshuffle on wrap (astronomically unlikely to produce identical order)")
	}
}

// ─── JumpToTrack ──────────────────────────────────────────────────────────────

func TestJumpToTrack_FindsTrack(t *testing.T) {
	tracks := makeTracks(4)
	p := New(tracks)
	p.SetFirst()

	ok := p.JumpToTrack(2)
	if !ok {
		t.Fatal("JumpToTrack(2) returned false")
	}
	if p.Current() != tracks[2] {
		t.Errorf("after jump, Current() is wrong track")
	}
}

func TestJumpToTrack_ReturnsFalseForMissingIndex(t *testing.T) {
	p := New(makeTracks(3))
	if p.JumpToTrack(99) {
		t.Error("JumpToTrack with out-of-range index should return false")
	}
}

func TestJumpToTrack_RecordsHistory(t *testing.T) {
	p := New(makeTracks(3))
	p.SetFirst()
	p.JumpToTrack(2)
	if len(p.History()) != 2 {
		t.Errorf("want 2 history entries, got %d", len(p.History()))
	}
}

// ─── History cap ─────────────────────────────────────────────────────────────

func TestHistory_CappedAt15(t *testing.T) {
	// Use 20 tracks; play them all in order.
	p := New(makeTracks(20))
	p.SetRepeat(true)
	p.SetFirst()
	for i := 0; i < 19; i++ {
		p.Next()
	}

	h := p.History()
	if len(h) > maxHistory {
		t.Errorf("history len = %d, want <= %d", len(h), maxHistory)
	}
}

func TestHistory_OldestDropped(t *testing.T) {
	p := New(makeTracks(20))
	p.SetRepeat(true)
	p.SetFirst()
	for i := 0; i < 19; i++ {
		p.Next()
	}

	h := p.History()
	// Newest entry should be the 20th track (index 19).
	newest := h[len(h)-1]
	if newest.TrackIndex != 19 {
		t.Errorf("newest history entry TrackIndex = %d, want 19", newest.TrackIndex)
	}
}

// ─── SetShuffle ───────────────────────────────────────────────────────────────

func TestSetShuffle_PermutesOrder(t *testing.T) {
	// With 8 tracks the probability of the shuffle producing the identity
	// permutation is 1/8! ≈ 0.000025 — effectively impossible.
	p := New(makeTracks(8))
	p.SetFirst()
	p.SetShuffle(true)

	natural := true
	for i, v := range p.order {
		if v != i {
			natural = false
			break
		}
	}
	if natural {
		t.Error("shuffle produced natural order (astronomically unlikely unless broken)")
	}
}

func TestSetShuffle_AllTracksPresent(t *testing.T) {
	n := 8
	p := New(makeTracks(n))
	p.SetFirst()
	p.SetShuffle(true)

	seen := make(map[int]bool)
	for _, v := range p.order {
		seen[v] = true
	}
	if len(seen) != n {
		t.Errorf("after shuffle, order has %d unique values, want %d", len(seen), n)
	}
}

func TestSetShuffle_CurrentTrackRemainsAtCursor(t *testing.T) {
	tracks := makeTracks(8)
	p := New(tracks)
	p.SetFirst()
	p.Next() // cursor = 1, playing tracks[1]
	currentBefore := p.Current()

	p.SetShuffle(true)

	if p.Current() != currentBefore {
		t.Errorf("current track changed after enabling shuffle")
	}
	if p.order[p.cursor] != 1 {
		t.Errorf("cursor position %d does not point to track 1, got %d", p.cursor, p.order[p.cursor])
	}
}

func TestSetShuffle_OffRestoresNaturalOrder(t *testing.T) {
	p := New(makeTracks(8))
	p.SetFirst()
	p.Next() // cursor = 1
	p.SetShuffle(true)
	p.SetShuffle(false)

	for i, v := range p.order {
		if v != i {
			t.Errorf("order[%d] = %d after unshuffle, want %d", i, v, i)
		}
	}
}

func TestSetShuffle_OffCursorAtNaturalIndex(t *testing.T) {
	p := New(makeTracks(8))
	p.SetFirst()
	p.Next() // currently playing track index 1
	p.SetShuffle(true)
	p.SetShuffle(false)

	// After restoring natural order, cursor should be at position 1
	// (natural order: position == track index).
	if p.cursor != 1 {
		t.Errorf("cursor after unshuffle = %d, want 1", p.cursor)
	}
}

func TestSetShuffle_NoopIfAlreadySet(t *testing.T) {
	p := New(makeTracks(4))
	p.SetFirst()
	p.SetShuffle(true)
	orderAfterFirst := orderOf(p)

	p.SetShuffle(true) // second call should be a no-op
	orderAfterSecond := orderOf(p)

	for i := range orderAfterFirst {
		if orderAfterFirst[i] != orderAfterSecond[i] {
			t.Errorf("order changed on duplicate SetShuffle(true)")
		}
	}
}

// ─── SetRepeat ────────────────────────────────────────────────────────────────

func TestSetRepeat_TogglesFlag(t *testing.T) {
	p := New(makeTracks(2))
	if p.Repeat() {
		t.Error("repeat should be off by default")
	}
	p.SetRepeat(true)
	if !p.Repeat() {
		t.Error("repeat should be on after SetRepeat(true)")
	}
	p.SetRepeat(false)
	if p.Repeat() {
		t.Error("repeat should be off after SetRepeat(false)")
	}
}
