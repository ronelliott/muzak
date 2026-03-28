package library

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"strings"

	"github.com/hirochachacha/go-smb2"

	"github.com/ronelliott/muzak/audio"
)

// IsSMBPath reports whether s is an SMB URL (smb:// scheme, case-insensitive).
func IsSMBPath(s string) bool {
	return strings.HasPrefix(strings.ToLower(s), "smb://")
}

// smbConfig holds the parsed components of an SMB URL.
type smbConfig struct {
	host    string // host or host:port
	user    string
	pass    string
	share   string
	subpath string // path within the share, may be empty
}

// parseSMBURL parses smb://user:pass@host/share[/subpath] into its components.
func parseSMBURL(rawURL string) (smbConfig, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return smbConfig{}, fmt.Errorf("parse SMB URL: %w", err)
	}
	if strings.ToLower(u.Scheme) != "smb" {
		return smbConfig{}, fmt.Errorf("not an SMB URL: %s", rawURL)
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "445"
	}

	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}

	// Path is /share[/subpath...] — split off the share name.
	trimmed := strings.TrimPrefix(u.Path, "/")
	parts := strings.SplitN(trimmed, "/", 2)
	share := parts[0]
	subpath := ""
	if len(parts) == 2 {
		subpath = parts[1]
	}

	if share == "" {
		return smbConfig{}, fmt.Errorf("SMB URL missing share name: %s", rawURL)
	}

	return smbConfig{
		host:    net.JoinHostPort(host, port),
		user:    user,
		pass:    pass,
		share:   share,
		subpath: subpath,
	}, nil
}

// connectSMB dials the SMB server, authenticates, and mounts the share.
// The caller must call the returned teardown function when done.
func connectSMB(cfg smbConfig) (*smb2.Share, func(), error) {
	conn, err := net.Dial("tcp", cfg.host)
	if err != nil {
		return nil, nil, fmt.Errorf("dial SMB %s: %w", cfg.host, err)
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.user,
			Password: cfg.pass,
		},
	}

	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("SMB auth %s: %w", cfg.host, err)
	}

	share, err := session.Mount(cfg.share)
	if err != nil {
		session.Logoff() //nolint:errcheck
		conn.Close()
		return nil, nil, fmt.Errorf("mount SMB share %s: %w", cfg.share, err)
	}

	teardown := func() {
		share.Umount()        //nolint:errcheck
		session.Logoff()      //nolint:errcheck
		conn.Close()
	}
	return share, teardown, nil
}

// scanSMB scans an SMB URL recursively for audio files and returns tracks.
// The cache key for each file is its full smb:// URL.
func scanSMB(smbURL string) ([]*Track, error) {
	cfg, err := parseSMBURL(smbURL)
	if err != nil {
		return nil, err
	}

	share, teardown, err := connectSMB(cfg)
	if err != nil {
		return nil, err
	}
	defer teardown()

	old := loadDiskCache()
	newCache := &diskCache{
		Version: cacheVersion,
		Entries: make(map[string]*cacheEntry, len(old.Entries)),
	}

	// Preserve existing non-SMB entries and SMB entries from other sources.
	for k, v := range old.Entries {
		newCache.Entries[k] = v
	}

	root := cfg.subpath
	if root == "" {
		root = "."
	}

	// Build the URL prefix for cache keys: smb://user:pass@host/share/
	urlPrefix := smbURLPrefix(cfg)

	var tracks []*Track

	err = fs.WalkDir(share.DirFS(root), ".", func(relPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		// Full SMB URL for this file.
		filePath := relPath
		if root != "." {
			filePath = root + "/" + relPath
		}
		fileURL := urlPrefix + filePath

		ext := strings.ToLower(filePathExt(relPath))
		switch ext {
		case ".flac", ".wav":
			ts, entry := cachedOrScanSMBFile(fileURL, share, filePath, old)
			tracks = append(tracks, ts...)
			if entry != nil {
				newCache.Entries[fileURL] = entry
			}
		case ".zip":
			ts, entry := cachedOrScanSMBZip(fileURL, share, filePath, old)
			tracks = append(tracks, ts...)
			if entry != nil {
				newCache.Entries[fileURL] = entry
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk SMB share: %w", err)
	}

	saveDiskCache(newCache)
	return deduplicate(tracks), nil
}

// smbURLPrefix returns "smb://user:pass@host/share/" for use as a cache key prefix.
func smbURLPrefix(cfg smbConfig) string {
	host, port, _ := net.SplitHostPort(cfg.host)
	authority := host
	if port != "445" {
		authority = net.JoinHostPort(host, port)
	}
	creds := ""
	if cfg.user != "" {
		creds = url.UserPassword(cfg.user, cfg.pass).String() + "@"
	}
	return fmt.Sprintf("smb://%s%s/%s/", creds, authority, cfg.share)
}

// filePathExt returns the extension of a slash-separated path.
func filePathExt(p string) string {
	for i := len(p) - 1; i >= 0 && p[i] != '/'; i-- {
		if p[i] == '.' {
			return p[i:]
		}
	}
	return ""
}

// cachedOrScanSMBFile returns tracks for a plain audio file on an SMB share,
// using the cache when the file's fingerprint matches.
func cachedOrScanSMBFile(fileURL string, share *smb2.Share, sharePath string, c *diskCache) ([]*Track, *cacheEntry) {
	info, err := share.Stat(sharePath)
	if err != nil {
		return nil, nil
	}

	if entry, ok := c.Entries[fileURL]; ok &&
		entry.ModTime == info.ModTime().UnixNano() &&
		entry.Size == info.Size() {
		return reconstructTracks(fileURL, entry), entry
	}

	f, err := share.Open(sharePath)
	if err != nil {
		return nil, nil
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, nil
	}

	format, ok := formatFromName(sharePath)
	if !ok {
		return nil, nil
	}

	rs := bytes.NewReader(data)
	t := &Track{
		Path:   fileURL,
		Format: format,
		Opener: smbFileOpener(fileURL),
	}
	populateMetadata(t, rs)
	rs.Seek(0, io.SeekStart) //nolint:errcheck
	t.Duration = decodeDuration(rs, format)

	entry := buildCacheEntry(info, []*Track{t})
	return []*Track{t}, entry
}

// cachedOrScanSMBZip returns tracks for a ZIP archive on an SMB share,
// using the cache when the archive's fingerprint matches.
func cachedOrScanSMBZip(fileURL string, share *smb2.Share, sharePath string, c *diskCache) ([]*Track, *cacheEntry) {
	info, err := share.Stat(sharePath)
	if err != nil {
		return nil, nil
	}

	if entry, ok := c.Entries[fileURL]; ok &&
		entry.ModTime == info.ModTime().UnixNano() &&
		entry.Size == info.Size() {
		return reconstructTracks(fileURL, entry), entry
	}

	f, err := share.Open(sharePath)
	if err != nil {
		return nil, nil
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return nil, nil
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil
	}

	var tracks []*Track
	for _, zf := range zr.File {
		format, ok := formatFromName(zf.Name)
		if !ok {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			continue
		}
		entryData, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		entryName := zf.Name
		rs := bytes.NewReader(entryData)
		t := &Track{
			Path:     fileURL,
			ZipEntry: entryName,
			Format:   format,
			Opener:   smbZipOpener(fileURL, entryName),
		}
		populateMetadata(t, rs)
		rs.Seek(0, io.SeekStart) //nolint:errcheck
		t.Duration = decodeDuration(rs, format)
		tracks = append(tracks, t)
	}

	if len(tracks) == 0 {
		return nil, nil
	}

	entry := buildCacheEntry(info, tracks)
	return tracks, entry
}

// smbFileOpener returns an Opener that connects to the SMB share and buffers
// the file into memory for seekable playback.
func smbFileOpener(fileURL string) audio.Opener {
	return func() (io.ReadSeekCloser, error) {
		cfg, err := parseSMBURL(fileURL)
		if err != nil {
			return nil, err
		}

		share, teardown, err := connectSMB(cfg)
		if err != nil {
			return nil, err
		}
		defer teardown()

		sharePath := cfg.subpath
		f, err := share.Open(sharePath)
		if err != nil {
			return nil, fmt.Errorf("open SMB file %s: %w", fileURL, err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read SMB file %s: %w", fileURL, err)
		}
		return nopCloser{bytes.NewReader(data)}, nil
	}
}

// smbZipOpener returns an Opener that fetches a ZIP from an SMB share and
// extracts the named entry into memory for seekable playback.
func smbZipOpener(zipURL, entryName string) audio.Opener {
	return func() (io.ReadSeekCloser, error) {
		cfg, err := parseSMBURL(zipURL)
		if err != nil {
			return nil, err
		}

		share, teardown, err := connectSMB(cfg)
		if err != nil {
			return nil, err
		}
		defer teardown()

		f, err := share.Open(cfg.subpath)
		if err != nil {
			return nil, fmt.Errorf("open SMB zip %s: %w", zipURL, err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read SMB zip %s: %w", zipURL, err)
		}

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("parse SMB zip %s: %w", zipURL, err)
		}

		for _, zf := range zr.File {
			if zf.Name != entryName {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return nil, fmt.Errorf("open zip entry %s: %w", entryName, err)
			}
			entryData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("read zip entry %s: %w", entryName, err)
			}
			return nopCloser{bytes.NewReader(entryData)}, nil
		}
		return nil, fmt.Errorf("entry %q not found in %s", entryName, zipURL)
	}
}
