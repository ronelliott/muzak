# muzak

A no-frills terminal music player for macOS. Point it at a directory, it plays your music. No library management, no database, no daemon, no configuration file to learn. Just a keyboard-driven interface that stays out of the way.

Plays FLAC and WAV files, including tracks stored inside ZIP archives.

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
go install github.com/ronelliott/muzak@latest
```

Or build from source:

```sh
git clone https://github.com/ronelliott/muzak
cd muzak
go build -o muzak .
```

Requires Go 1.26+. No external system dependencies (audio output uses CoreAudio via CGo, which is part of the macOS SDK).

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
| `[` | Rewind 10 seconds |
| `]` | Fast-forward 10 seconds |
| `+` | Volume up |
| `-` | Volume down |
| `s` | Toggle shuffle |
| `r` | Toggle repeat |
| `l` | Open / close library |
| `↑` / `↓` | Navigate history or library |
| `Enter` | Play from selected track |
| `Esc` | Close library |
| `q` | Quit |

## Library overlay

Press `l` to open the library overlay, which lists all discovered tracks. Type to filter by artist or title. Press `Enter` to start playing from the selected track. Press `Esc` or `l` again to return to the history view.

## History

The last 15 played tracks are shown above the now-playing bar, newest at the bottom. Use `↑` / `↓` to navigate the list and `Enter` to jump back to a track and resume playing forward from that point.

## Status indicators

The now-playing bar shows:
- `[S]` when shuffle is active
- `[R]` when repeat is active
- Current position and total duration
- Current volume percentage

## Cache

Track metadata is cached in `$UserCacheDir/muzak/library.json` (typically `~/Library/Caches/muzak/library.json` on macOS). The cache is keyed by file path, modification time, and size. Modified or new files are re-scanned automatically; the cache is updated on every run.

## Architecture

```
audio/           Abstract playback backend interface
audio/beepbackend/  macOS implementation via gopxl/beep + oto/CoreAudio
library/         File scanner, metadata reader, track cache
playlist/        Playback queue, shuffle, repeat, history
ui/              BubbleTea terminal interface
```
