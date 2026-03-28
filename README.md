# muzak

A no-frills terminal music player for macOS, Linux, and Windows. Point it at a directory, it plays your music. No library management, no database, no daemon, no configuration file to learn. Just a keyboard-driven interface that stays out of the way.

Plays FLAC and WAV files, including tracks stored inside ZIP archives.

> **Linux and Windows support note:** Linux and Windows binaries are tested via automated CI only and have not been verified on actual hardware. Bug reports welcome.

## Features

- Plays `.flac` and `.wav` files
- Plays audio directly from `.zip` archives without extracting
- Reads metadata tags (artist, title, album) for display
- Shuffle and repeat modes
- 15-entry play history with jump-to navigation
- Searchable library overlay
- Library metadata cache for fast startup on repeat runs
- Ignores hidden files and directories

## Installation

```sh
go install github.com/ronelliott/muzak/cmd@latest
```

Or build from source:

```sh
git clone https://github.com/ronelliott/muzak
cd muzak
go build -o muzak ./cmd/
```

Requires Go 1.26+.

- **macOS:** No external system dependencies — audio uses CoreAudio via CGo, which is part of the macOS SDK.
- **Linux:** Requires ALSA (`libasound2-dev` on Debian/Ubuntu, `alsa-lib-devel` on Fedora/RHEL). Homebrew on Linux installs this automatically.
- **Windows:** No external system dependencies — audio uses WASAPI via pure Go bindings. Requires Windows Terminal or PowerShell 7+ for the terminal UI; the legacy CMD prompt is not supported.

## Usage

```sh
muzak [directory ...]
```

Pass one or more directories to scan. If no directories are given, the current directory is scanned.

```sh
muzak ~/Music
muzak ~/Music ~/Downloads/albums
muzak .
```

Playback starts automatically with the first track found.

## Controls

| Key | Action |
|-----|--------|
| `p` | Pause / play |
| `,` | Previous track |
| `.` | Next track |
| `[` | Rewind 10 seconds |
| `]` | Fast-forward 10 seconds |
| `+` | Volume up |
| `-` | Volume down |
| `s` | Toggle shuffle |
| `r` | Toggle repeat |
| `Esc` | Open / close library |
| `↑` / `↓` | Navigate history or library |
| `Enter` | Play from selected track |
| `q` | Quit |

## Library overlay

Press `Esc` to open the library overlay, which lists all discovered tracks. Type to filter by artist or title. Press `Enter` to start playing from the selected track. Press `Esc` again to return to the history view.

## History

The last 15 played tracks are shown above the now-playing bar, newest at the bottom. Use `↑` / `↓` to navigate the list and `Enter` to jump back to a track and resume playing forward from that point.

## Status indicators

The now-playing bar shows:
- `▶` when playing, `⏸` when paused, `⏹` when the playlist has ended
- `[S]` when shuffle is active
- `[R]` when repeat is active
- Current position and total duration
- Current volume percentage

## Cache

Track metadata is cached in `$UserCacheDir/muzak/library.json` (typically `~/Library/Caches/muzak/library.json` on macOS). The cache is keyed by file path, modification time, and size. Modified or new files are re-scanned automatically; the cache is updated on every run.

## Architecture

```
audio/              Abstract playback backend interface
audio/beepbackend/  Platform implementation via gopxl/beep + oto
                    (CoreAudio on macOS, ALSA on Linux, WASAPI on Windows)
library/            File scanner, metadata reader, track cache
playlist/           Playback queue, shuffle, repeat, history
ui/                 BubbleTea terminal interface
```
