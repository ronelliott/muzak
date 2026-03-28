package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ronelliott/muzak/audio/beepbackend"
	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/muzak/playlist"
	"github.com/ronelliott/muzak/ui"
)

// version is set at build time via -ldflags "-X main.version=<value>".
// Falls back to the embedded VCS commit hash for local builds.
var version = "dev"

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" {
					if len(s.Value) > 7 {
						version = s.Value[:7]
					} else {
						version = s.Value
					}
					break
				}
			}
		}
	}
}

const usage = `muzak - terminal music player

Usage:
  muzak [flags] [directory...]

Flags:
  --version   Print version and exit
  --help      Print this help and exit

Controls:
  p        Pause / play
  [ / ]    Rewind / forward
  s        Shuffle
  r        Repeat
  + / -    Volume up / down
  l        Library overlay
  ↑ / ↓    Navigate
  ↵        Play selected
  q        Quit`

func main() {
	args := os.Args[1:]

	for _, arg := range args {
		switch arg {
		case "--version":
			fmt.Println(version)
			return
		case "--help":
			fmt.Println(usage)
			return
		}
	}

	dirs := args
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
