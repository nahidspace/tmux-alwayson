package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClaudeValidSession(t *testing.T) {
	home := t.TempDir()
	c := Claude{Home: home}

	projectDir := filepath.Join(home, ".claude", "projects", sanitizeCwd("/home/kali"))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Real, resumable session.
	realID := "e129076c-9381-46cd-bd6a-051e989a05a3"
	if err := os.WriteFile(filepath.Join(projectDir, realID+".jsonl"), []byte(`{"type":"summary"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Phantom session: SessionStart hook fired, transcript never written.
	phantomID := "d1d6516d-dde1-4263-86e2-927439f176a3"

	if !c.ValidSession(realID, "/home/kali") {
		t.Errorf("real session %s should be valid", realID)
	}
	if c.ValidSession(phantomID, "/home/kali") {
		t.Errorf("phantom session %s should NOT be valid (no transcript)", phantomID)
	}
	if c.ValidSession(realID, "/some/other/cwd") {
		t.Errorf("real session under wrong cwd should NOT be valid")
	}

	got, ok := c.NewestSession("/home/kali")
	if !ok || got != realID {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", got, ok, realID)
	}
}

func TestClaudeNewestSessionPicksMostRecent(t *testing.T) {
	home := t.TempDir()
	c := Claude{Home: home}
	projectDir := filepath.Join(home, ".claude", "projects", sanitizeCwd("/home/kali"))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	older := filepath.Join(projectDir, "older.jsonl")
	newer := filepath.Join(projectDir, "newer.jsonl")
	if err := os.WriteFile(older, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(newer, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := c.NewestSession("/home/kali")
	if !ok || got != "newer" {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", got, ok, "newer")
	}
}

func TestClaudeMatches(t *testing.T) {
	cases := map[string]bool{
		"claude --resume abc":                   true,
		"/usr/bin/claude --dangerously-skip":    true,
		"claude":                                true,
		"opencode run":                          false,
		"claude-code-guide-something-unrelated": false,
	}
	c := Claude{}
	for cmdline, want := range cases {
		if got := c.Matches(cmdline); got != want {
			t.Errorf("Matches(%q) = %v, want %v", cmdline, got, want)
		}
	}
}
