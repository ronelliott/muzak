package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ronelliott/muzak/audio/beepbackend"
	"github.com/ronelliott/muzak/library"
	"github.com/ronelliott/muzak/playlist"
	"github.com/ronelliott/muzak/ui"
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

func usageText() string {
	name := filepath.Base(os.Args[0])
	return fmt.Sprintf(`%s - terminal music player

Usage:
  %s [flags] [--] [directory...]

Flags:
  --version   Print version and exit
  --help      Print this help and exit

Controls:
  p            Pause / play
  [ / ]        Rewind / forward 10s
  s            Shuffle
  r            Repeat
  + / -        Volume up / down
  l / esc      Toggle library overlay
  ↑ / ↓        Navigate
  ↵            Play selected
  q / ctrl+c   Quit`, name, name)
}

func main() {
	args := os.Args[1:]
	var dirs []string
	pastFlags := false

	for _, arg := range args {
		if pastFlags || arg == "--" {
			if arg != "--" {
				dirs = append(dirs, arg)
			}
			pastFlags = true
			continue
		}
		switch arg {
		case "--version":
			fmt.Println(getVersion())
			return
		case "--help":
			fmt.Println(usageText())
			return
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n\n%s\n", arg, usageText())
				os.Exit(1)
			}
			dirs = append(dirs, arg)
		}
	}

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
