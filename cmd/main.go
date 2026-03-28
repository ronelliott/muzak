package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
	cmdlibrary "github.com/ronelliott/muzak/cmd/library"
	"github.com/ronelliott/muzak/audio/beepbackend"
	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/muzak/playlist"
	"github.com/ronelliott/muzak/ui"
	"github.com/ronelliott/snek"
)

// version is set at build time via -ldflags "-X main.version=<value>".
// Falls back to the embedded VCS commit hash when --version is invoked.
var version = "dev"

func getVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				if len(s.Value) > 7 {
					return s.Value[:7]
				}
				return s.Value
			}
		}
	}
	return version
}

func main() {
	snek.RunExit(
		snek.NewConfig(),
		snek.WithUse("muzak [directory...]"),
		snek.WithShort("Terminal music player"),
		snek.WithLong(`muzak - terminal music player

Controls:
  p            Pause / play
  [ / ]        Rewind / forward 10s
  s            Shuffle
  r            Repeat
  + / -        Volume up / down
  esc          Toggle library overlay
  ↑ / ↓        Navigate
  ↵            Play selected
  q / ctrl+c   Quit`),
		snek.WithVersion(getVersion()),
		snek.WithSimpleRunE(runPlay),
		snek.WithSubCommandGenerator(cmdlibrary.NewCommand),
	)
}

func runPlay(args []string) error {
	var tracks []*library.Track

	if len(args) > 0 {
		// Explicit directories — scan and ignore stored library.
		fmt.Fprintln(os.Stderr, "Scanning for audio files…")
		var err error
		tracks, err = library.Scan(args)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}
	} else {
		// No args — use stored library sources.
		sources := library.LoadSources()
		if len(sources.Paths) > 0 {
			tracks = library.LoadFromCache(sources.Paths)
		} else {
			// Fall back to scanning current directory.
			fmt.Fprintln(os.Stderr, "Scanning for audio files…")
			var err error
			tracks, err = library.Scan([]string{"."})
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}
		}
	}

	if len(tracks) == 0 {
		return fmt.Errorf("no audio files found")
	}
	fmt.Fprintf(os.Stderr, "Found %d track(s).\n", len(tracks))

	backend := beepbackend.New()
	if err := backend.Init(); err != nil {
		return fmt.Errorf("audio init: %w", err)
	}
	defer backend.Close()

	pl := playlist.New(tracks)
	model := ui.NewModel(backend, pl)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("ui: %w", err)
	}
	return nil
}
