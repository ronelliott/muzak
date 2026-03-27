package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ronelliott/muzak/audio/beepbackend"
	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/muzak/playlist"
	"github.com/ronelliott/muzak/ui"
)

func main() {
	dirs := os.Args[1:]
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	fmt.Fprintln(os.Stderr, "Scanning for audio files…")
	tracks, err := library.Scan(dirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(1)
	}
	if len(tracks) == 0 {
		fmt.Fprintln(os.Stderr, "No audio files found in the given directories.")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Found %d track(s).\n", len(tracks))

	backend := beepbackend.New()
	if err := backend.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "audio init error: %v\n", err)
		os.Exit(1)
	}
	defer backend.Close()

	pl := playlist.New(tracks)
	model := ui.NewModel(backend, pl)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ui error: %v\n", err)
		os.Exit(1)
	}
}
