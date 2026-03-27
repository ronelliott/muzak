package library

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// minimalWAV returns the bytes of a valid stereo 44100 Hz 16-bit PCM WAV file
// containing 100 samples of silence. Small enough to be fast; valid enough for
// beepwav.Decode to parse.
func minimalWAV() []byte {
	const (
		sampleRate    = 44100
		numChannels   = 2
		bitsPerSample = 16
		numSamples    = 100
	)
	dataSize := uint32(numSamples * numChannels * bitsPerSample / 8)

	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize)) //nolint:errcheck
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))                                         //nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(1))                                          //nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(numChannels))                                //nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))                                 //nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*numChannels*bitsPerSample/8))     //nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(numChannels*bitsPerSample/8))                //nolint:errcheck
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))                              //nolint:errcheck
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, dataSize) //nolint:errcheck
	buf.Write(make([]byte, dataSize))
	return buf.Bytes()
}

// writeWAVFile creates a WAV file at dir/name and returns its full path.
func writeWAVFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, minimalWAV(), 0o644); err != nil {
		t.Fatalf("writeWAVFile: %v", err)
	}
	return path
}

// writeZipWithWAVs creates a ZIP at dir/zipName containing one WAV entry per
// name in entries. Returns the ZIP path.
func writeZipWithWAVs(t *testing.T, dir, zipName string, entries []string) string {
	t.Helper()
	zipPath := filepath.Join(dir, zipName)

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, entry := range entries {
		e, err := w.Create(entry)
		if err != nil {
			t.Fatalf("zip create entry %s: %v", entry, err)
		}
		if _, err := e.Write(minimalWAV()); err != nil {
			t.Fatalf("zip write entry %s: %v", entry, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return zipPath
}
