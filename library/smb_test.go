package library

import "testing"

func TestIsSMBPath(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"smb://user:pass@server/share", true},
		{"SMB://user:pass@server/share", true},
		{"smb://server/share/path", true},
		{"/local/path", false},
		{"./relative", false},
		{"", false},
		{"ftp://server/share", false},
	}
	for _, c := range cases {
		if got := IsSMBPath(c.input); got != c.want {
			t.Errorf("IsSMBPath(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseSMBURL(t *testing.T) {
	cases := []struct {
		input    string
		host     string
		user     string
		pass     string
		share    string
		subpath  string
		wantErr  bool
	}{
		{
			input:   "smb://user:pass@server/share/music",
			host:    "server:445",
			user:    "user",
			pass:    "pass",
			share:   "share",
			subpath: "music",
		},
		{
			input:   "smb://user:pass@server:4445/share",
			host:    "server:4445",
			user:    "user",
			pass:    "pass",
			share:   "share",
			subpath: "",
		},
		{
			input:   "smb://server/share/deep/path",
			host:    "server:445",
			user:    "",
			pass:    "",
			share:   "share",
			subpath: "deep/path",
		},
		{
			input:   "smb://server/share",
			host:    "server:445",
			share:   "share",
			subpath: "",
		},
		{
			input:   "smb://server/",
			wantErr: true, // missing share
		},
		{
			input:   "/not/smb",
			wantErr: true,
		},
	}

	for _, c := range cases {
		cfg, err := parseSMBURL(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseSMBURL(%q): expected error, got none", c.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSMBURL(%q): unexpected error: %v", c.input, err)
			continue
		}
		if cfg.host != c.host {
			t.Errorf("parseSMBURL(%q).host = %q, want %q", c.input, cfg.host, c.host)
		}
		if cfg.user != c.user {
			t.Errorf("parseSMBURL(%q).user = %q, want %q", c.input, cfg.user, c.user)
		}
		if cfg.pass != c.pass {
			t.Errorf("parseSMBURL(%q).pass = %q, want %q", c.input, cfg.pass, c.pass)
		}
		if cfg.share != c.share {
			t.Errorf("parseSMBURL(%q).share = %q, want %q", c.input, cfg.share, c.share)
		}
		if cfg.subpath != c.subpath {
			t.Errorf("parseSMBURL(%q).subpath = %q, want %q", c.input, cfg.subpath, c.subpath)
		}
	}
}

func TestSMBURLPrefix(t *testing.T) {
	cases := []struct {
		cfg  smbConfig
		want string
	}{
		{
			cfg:  smbConfig{host: "server:445", user: "user", pass: "pass", share: "music"},
			want: "smb://user:pass@server/music/",
		},
		{
			cfg:  smbConfig{host: "server:4445", user: "user", pass: "pass", share: "music"},
			want: "smb://user:pass@server:4445/music/",
		},
		{
			cfg:  smbConfig{host: "server:445", share: "music"},
			want: "smb://server/music/",
		},
	}

	for _, c := range cases {
		got := smbURLPrefix(c.cfg)
		if got != c.want {
			t.Errorf("smbURLPrefix(%+v) = %q, want %q", c.cfg, got, c.want)
		}
	}
}

func TestSourcesAdd_SMBPath(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())

	s := LoadSources()
	smbURL := "smb://user:pass@server/share/music"
	if err := s.Add(smbURL); err != nil {
		t.Fatalf("Add SMB URL: %v", err)
	}
	if len(s.Paths) != 1 || s.Paths[0] != smbURL {
		t.Errorf("want [%s], got %v", smbURL, s.Paths)
	}
}

func TestSourcesAdd_SMBDeduplication(t *testing.T) {
	t.Setenv("MUZAK_CONFIG_DIR", t.TempDir())

	s := LoadSources()
	smbURL := "smb://user:pass@server/share/music"
	s.Add(smbURL) //nolint:errcheck
	s.Add(smbURL) //nolint:errcheck

	if len(s.Paths) != 1 {
		t.Errorf("duplicate SMB add should not grow list: got %v", s.Paths)
	}
}
