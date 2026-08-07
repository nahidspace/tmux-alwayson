package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeValidSession(t *testing.T) {
	requireSQLite3(t)
	home := t.TempDir()
	dbDir := filepath.Join(home, ".local", "share", "opencode")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	makeTestDB(t, dbDir, "opencode.db", `
		CREATE TABLE session (id TEXT PRIMARY KEY, directory TEXT, time_updated INTEGER);
		INSERT INTO session VALUES ('ses_realabc123', '/home/kali', 100);
		INSERT INTO session VALUES ('ses_olderxyz789', '/home/kali', 50);
	`)

	o := OpenCode{Home: home}

	if !o.ValidSession("ses_realabc123", "/home/kali") {
		t.Error("real session should be valid")
	}
	if o.ValidSession("ses_doesnotexist", "/home/kali") {
		t.Error("nonexistent session should not be valid")
	}
	if o.ValidSession("ses_realabc123", "/wrong/cwd") {
		t.Error("session under wrong cwd should not be valid")
	}
	if o.ValidSession("not-even-ses-shaped", "/home/kali") {
		t.Error("malformed id should be rejected before ever touching the db")
	}

	newest, ok := o.NewestSession("/home/kali")
	if !ok || newest != "ses_realabc123" {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", newest, ok, "ses_realabc123")
	}
}

func TestOpenCodeMatches(t *testing.T) {
	o := OpenCode{}
	cases := map[string]bool{
		"opencode -s ses_abc":    true,
		"/usr/bin/opencode":      true,
		"opencode run something": false, // LSP subprocess, not a session
		"claude --resume abc":    false,
	}
	for cmdline, want := range cases {
		if got := o.Matches(cmdline); got != want {
			t.Errorf("Matches(%q) = %v, want %v", cmdline, got, want)
		}
	}
}
