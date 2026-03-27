package ui

import (
	"testing"
	"time"
)

// ─── fmtDur ──────────────────────────────────────────────────────────────────

func TestFmtDur(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0:00"},
		{time.Second, "0:01"},
		{59 * time.Second, "0:59"},
		{time.Minute, "1:00"},
		{time.Minute + 5*time.Second, "1:05"},
		{3*time.Minute + 45*time.Second, "3:45"},
		{61*time.Minute + 1*time.Second, "61:01"},
		{-time.Second, "0:00"}, // negative clamped to zero
	}

	for _, tt := range tests {
		got := fmtDur(tt.d)
		if got != tt.want {
			t.Errorf("fmtDur(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// ─── truncate ────────────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},      // shorter than limit — unchanged
		{"hello", 5, "hello"},       // exactly at limit — unchanged
		{"hello", 4, "hel…"},         // n=4 → 3 runes + "…"
		{"hello", 1, "…"},           // single char limit
		{"hello", 0, ""},            // zero width
		{"", 5, ""},                 // empty string
		{"héllo", 4, "hél…"},        // multibyte runes counted correctly
		{"日本語テスト", 4, "日本語…"}, // CJK runes
	}

	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
