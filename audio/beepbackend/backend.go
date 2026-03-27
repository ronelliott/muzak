// Package beepbackend implements the audio.Backend interface using gopxl/beep
// with an oto/CoreAudio output on macOS.
package beepbackend

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gopxl/beep"
	beepflac "github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/speaker"
	beepwav "github.com/gopxl/beep/wav"

	"github.com/ronelliott/muzak/audio"
)

// targetSampleRate is the fixed output sample rate. All tracks are resampled
// to this rate so the speaker only needs to be initialized once.
const targetSampleRate = beep.SampleRate(44100)

// bufferSize is ~100 ms of latency at 44100 Hz — a good balance between
// responsiveness and glitch-free playback.
const bufferSize = 4410

// Backend is the beep-based audio backend.
type Backend struct{}

// New creates a new beep backend.
func New() *Backend { return &Backend{} }

// Init initializes the speaker at 44100 Hz.
func (b *Backend) Init() error {
	return speaker.Init(targetSampleRate, bufferSize)
}

// Stop clears all streams from the speaker immediately.
func (b *Backend) Stop() { speaker.Clear() }

// Close shuts down the speaker.
func (b *Backend) Close() { speaker.Close() }

// Load decodes the audio, builds the playback pipeline, and returns a Playback
// that is initially paused.
func (b *Backend) Load(opener audio.Opener, format audio.Format) (audio.Playback, error) {
	r, err := opener()
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	var streamer beep.StreamSeeker
	var streamFmt beep.Format

	switch format {
	case audio.FormatFLAC:
		streamer, streamFmt, err = beepflac.Decode(r)
	case audio.FormatWAV:
		streamer, streamFmt, err = beepwav.Decode(r)
	default:
		r.Close()
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		r.Close()
		return nil, fmt.Errorf("decode %s: %w", format, err)
	}

	done := make(chan struct{})
	vol := &volumeStreamer{s: streamer, vol: 1.0}

	p := &playback{
		streamer:  streamer,
		vol:       vol,
		streamFmt: streamFmt,
		done:      done,
		closer:    r,
	}
	p.ctrl = &beep.Ctrl{Paused: true, Streamer: p.buildChain()}

	return p, nil
}

// ─── playback ────────────────────────────────────────────────────────────────

type playback struct {
	streamer  beep.StreamSeeker
	vol       *volumeStreamer
	ctrl      *beep.Ctrl
	streamFmt beep.Format
	done      chan struct{}
	doneOnce  sync.Once
	addedOnce sync.Once
	closer    io.Closer
}

// buildChain constructs Seq(vol/resampler, callback) for ctrl.Streamer.
// Must be called while holding the speaker lock (or before the playback is
// added to the speaker).
func (p *playback) buildChain() beep.Streamer {
	var top beep.Streamer = p.vol
	if p.streamFmt.SampleRate != targetSampleRate {
		top = beep.Resample(4, p.streamFmt.SampleRate, targetSampleRate, p.vol)
	}
	return beep.Seq(top, beep.Callback(p.closeDone))
}

func (p *playback) closeDone() {
	p.doneOnce.Do(func() { close(p.done) })
}

func (p *playback) Play() {
	p.addedOnce.Do(func() { speaker.Play(p.ctrl) })
	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()
}

func (p *playback) Pause() {
	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
}

func (p *playback) IsPlaying() bool {
	speaker.Lock()
	paused := p.ctrl.Paused
	speaker.Unlock()
	return !paused
}

func (p *playback) Seek(d time.Duration) error {
	samples := int(float64(p.streamFmt.SampleRate) * d.Seconds())
	if samples < 0 {
		samples = 0
	}
	if n := p.streamer.Len(); samples >= n {
		samples = n - 1
		if samples < 0 {
			samples = 0
		}
	}

	speaker.Lock()
	defer speaker.Unlock()

	if err := p.streamer.Seek(samples); err != nil {
		return err
	}
	// Rebuild the chain so stale resampler buffers are discarded.
	p.vol.s = p.streamer
	p.ctrl.Streamer = p.buildChain()
	return nil
}

func (p *playback) Duration() time.Duration {
	return p.streamFmt.SampleRate.D(p.streamer.Len())
}

func (p *playback) Position() time.Duration {
	speaker.Lock()
	pos := p.streamer.Position()
	speaker.Unlock()
	return p.streamFmt.SampleRate.D(pos)
}

func (p *playback) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	speaker.Lock()
	p.vol.vol = v
	speaker.Unlock()
}

func (p *playback) Volume() float64 {
	speaker.Lock()
	v := p.vol.vol
	speaker.Unlock()
	return v
}

func (p *playback) Done() <-chan struct{} { return p.done }

func (p *playback) Close() {
	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
	p.closeDone()
	if p.closer != nil {
		p.closer.Close()
	}
}

// ─── volumeStreamer ───────────────────────────────────────────────────────────

type volumeStreamer struct {
	s   beep.Streamer
	vol float64
}

func (v *volumeStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = v.s.Stream(samples)
	for i := range samples[:n] {
		samples[i][0] *= v.vol
		samples[i][1] *= v.vol
	}
	return
}

func (v *volumeStreamer) Err() error { return v.s.Err() }
