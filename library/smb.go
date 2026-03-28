package library

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"path"
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
	host    string // host:port (always has port)
	user    string
	pass    string
	share   string
	subpath string // path within the share, may be empty
}

// parseSMBURL parses smb://user:pass@host/share[/subpath] into its components.
// The hostname is normalised to lowercase; subpath leading/trailing slashes are
// stripped; the default port 445 is appended when no port is specified.
func parseSMBURL(rawURL string) (smbConfig, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return smbConfig{}, fmt.Errorf("parse SMB URL: %w", err)
	}
	if strings.ToLower(u.Scheme) != "smb" {
		return smbConfig{}, fmt.Errorf("not an SMB URL: %s", rawURL)
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return smbConfig{}, fmt.Errorf("SMB URL missing hostname: %s", rawURL)
	}
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
		subpath = strings.Trim(parts[1], "/") // normalise away leading/trailing slashes
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
		share.Umount()   //nolint:errcheck
		session.Logoff() //nolint:errcheck
		conn.Close()
	}
	return share, teardown, nil
}

// scanSMB scans an SMB URL recursively for audio files and returns tracks.
// Discovered entries are written into newCache using redacted (password-free)
// cache keys; old provides fingerprints for cache reuse. The caller is
// responsible for saving newCache.
func scanSMB(smbURL string, old *diskCache, newCache *diskCache) ([]*Track, error) {
	cfg, err := parseSMBURL(smbURL)
	if err != nil {
		return nil, err
	}

	share, teardown, err := connectSMB(cfg)
	if err != nil {
		return nil, err
	}
	defer teardown()

	root := cfg.subpath
	if root == "" {
		root = "."
	}

	// Cache keys use the redacted prefix so credentials are not persisted to disk.
	redactedPrefix := smbRedactedURLPrefix(cfg)

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

		// Build the share-relative path and the redacted cache-key URL.
		var sharePath string
		if root == "." {
			sharePath = relPath
		} else {
			sharePath = path.Join(root, relPath)
		}
		fileURL := redactedPrefix + sharePath

		ext := strings.ToLower(filePathExt(relPath))
		switch ext {
		case ".flac", ".wav":
			ts, entry := cachedOrScanSMBFile(fileURL, share, sharePath, cfg, old)
			tracks = append(tracks, ts...)
			if entry != nil {
				newCache.Entries[fileURL] = entry
			}
		case ".zip":
			ts, entry := cachedOrScanSMBZip(fileURL, share, sharePath, cfg, old)
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

	return deduplicate(tracks), nil
}

// smbURLPrefix returns "smb://[user:pass@]host/share/" for internal use
// (e.g. building openers). Port 445 is omitted from the authority.
func smbURLPrefix(cfg smbConfig) string {
	authority := smbAuthority(cfg.host, true)
	creds := ""
	if cfg.user != "" {
		if cfg.pass != "" {
			creds = url.UserPassword(cfg.user, cfg.pass).String() + "@"
		} else {
			creds = url.User(cfg.user).String() + "@"
		}
	}
	return fmt.Sprintf("smb://%s%s/%s/", creds, authority, cfg.share)
}

// smbRedactedURLPrefix returns "smb://[user@]host/share/" with the password
// omitted. Used for cache keys and Track.Path to avoid persisting credentials.
func smbRedactedURLPrefix(cfg smbConfig) string {
	authority := smbAuthority(cfg.host, true)
	creds := ""
	if cfg.user != "" {
		creds = url.User(cfg.user).String() + "@"
	}
	return fmt.Sprintf("smb://%s%s/%s/", creds, authority, cfg.share)
}

// smbAuthority returns the host (and optional port) portion of an SMB URL
// authority. When omitDefault is true the default port 445 is stripped.
// IPv6 addresses are always wrapped in brackets.
func smbAuthority(hostPort string, omitDefault bool) string {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	if omitDefault && port == "445" {
		if strings.Contains(host, ":") { // IPv6
			return "[" + host + "]"
		}
		return host
	}
	return net.JoinHostPort(host, port)
}

// smbRedactedPath converts a full SMB URL (possibly with password) to its
// redacted canonical form — the same form used as a cache key.
func smbRedactedPath(rawURL string) string {
	cfg, err := parseSMBURL(rawURL)
	if err != nil {
		return rawURL
	}
	prefix := smbRedactedURLPrefix(cfg) // "smb://user@host/share/"
	if cfg.subpath != "" {
		return prefix + cfg.subpath
	}
	return strings.TrimSuffix(prefix, "/")
}

// SMBRedactPath returns the SMB URL with the password removed (username kept).
// Safe for use in user-facing messages and log output.
func SMBRedactPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.User == nil {
		return rawURL
	}
	if username := u.User.Username(); username != "" {
		u.User = url.User(username)
	} else {
		u.User = nil
	}
	return u.String()
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
func cachedOrScanSMBFile(fileURL string, share *smb2.Share, sharePath string, cfg smbConfig, c *diskCache) ([]*Track, *cacheEntry) {
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
		Opener: smbShareFileOpener(cfg, sharePath),
	}
	populateMetadata(t, rs)
	rs.Seek(0, io.SeekStart) //nolint:errcheck
	t.Duration = decodeDuration(rs, format)

	entry := buildCacheEntry(info, []*Track{t})
	return []*Track{t}, entry
}

// cachedOrScanSMBZip returns tracks for a ZIP archive on an SMB share,
// using the cache when the archive's fingerprint matches.
func cachedOrScanSMBZip(fileURL string, share *smb2.Share, sharePath string, cfg smbConfig, c *diskCache) ([]*Track, *cacheEntry) {
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
			Opener:   smbShareZipOpener(cfg, sharePath, entryName),
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

// smbShareFileOpener returns an Opener that connects using cfg and buffers the
// file at sharePath into memory for seekable playback. Used at scan time when
// credentials are already available in cfg.
func smbShareFileOpener(cfg smbConfig, sharePath string) audio.Opener {
	return func() (io.ReadSeekCloser, error) {
		share, teardown, err := connectSMB(cfg)
		if err != nil {
			return nil, err
		}
		defer teardown()

		f, err := share.Open(sharePath)
		if err != nil {
			return nil, fmt.Errorf("open SMB file %s: %w", sharePath, err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read SMB file %s: %w", sharePath, err)
		}
		return nopCloser{bytes.NewReader(data)}, nil
	}
}

// smbShareZipOpener returns an Opener that fetches a ZIP from an SMB share and
// extracts the named entry into memory for seekable playback. Used at scan time.
func smbShareZipOpener(cfg smbConfig, sharePath, entryName string) audio.Opener {
	return func() (io.ReadSeekCloser, error) {
		share, teardown, err := connectSMB(cfg)
		if err != nil {
			return nil, err
		}
		defer teardown()

		f, err := share.Open(sharePath)
		if err != nil {
			return nil, fmt.Errorf("open SMB zip %s: %w", sharePath, err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read SMB zip %s: %w", sharePath, err)
		}

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("parse SMB zip %s: %w", sharePath, err)
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
		return nil, fmt.Errorf("entry %q not found in %s", entryName, sharePath)
	}
}

// smbFileOpener returns an Opener for a cached SMB file whose path is in
// redacted (password-free) form. Credentials are looked up from the configured
// sources at open time so they are never stored in the cache.
func smbFileOpener(redactedFileURL string) audio.Opener {
	return func() (io.ReadSeekCloser, error) {
		fullURL := lookupSMBCredentials(redactedFileURL)
		cfg, err := parseSMBURL(fullURL)
		if err != nil {
			return nil, err
		}
		return smbShareFileOpener(cfg, cfg.subpath)()
	}
}

// smbZipOpener returns an Opener for a cached SMB ZIP entry whose path is in
// redacted form. Credentials are looked up from sources at open time.
func smbZipOpener(redactedZipURL, entryName string) audio.Opener {
	return func() (io.ReadSeekCloser, error) {
		fullURL := lookupSMBCredentials(redactedZipURL)
		cfg, err := parseSMBURL(fullURL)
		if err != nil {
			return nil, err
		}
		return smbShareZipOpener(cfg, cfg.subpath, entryName)()
	}
}

// lookupSMBCredentials finds the source in the sources list whose redacted
// prefix matches redactedPath and returns the full URL (with password)
// for that path. Returns redactedPath unchanged if no match is found.
func lookupSMBCredentials(redactedPath string) string {
	sources := LoadSources()
	for _, src := range sources.Paths {
		if !IsSMBPath(src) {
			continue
		}
		cfg, err := parseSMBURL(src)
		if err != nil {
			continue
		}
		prefix := smbRedactedURLPrefix(cfg)
		if strings.HasPrefix(redactedPath, prefix) {
			fullPrefix := smbURLPrefix(cfg)
			return fullPrefix + redactedPath[len(prefix):]
		}
	}
	return redactedPath
}
