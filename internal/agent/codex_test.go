package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRolloutFile(t *testing.T, path, id, cwd string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session_meta","id":"` + id + `","cwd":"` + cwd + `"}` + "\n" +
		`{"type":"message","role":"user","content":"hi"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCodexRolloutFallback(t *testing.T) {
	home := t.TempDir()
	sessionsRoot := filepath.Join(home, ".codex", "sessions")

	older := filepath.Join(sessionsRoot, "2026", "08", "07", "older-session.jsonl")
	writeRolloutFile(t, older, "older-session", "/home/kali")
	time.Sleep(10 * time.Millisecond)
	newer := filepath.Join(sessionsRoot, "2026", "08", "07", "newer-session.jsonl")
	writeRolloutFile(t, newer, "newer-session", "/home/kali")

	c := Codex{Home: home}

	// No state_*.sqlite present, so this exercises the rollout-file path.
	if !c.ValidSession("newer-session", "/home/kali") {
		t.Error("real rollout session should be valid")
	}
	if c.ValidSession("newer-session", "/wrong/cwd") {
		t.Error("session under wrong cwd should not be valid")
	}
	if c.ValidSession("no-such-session", "/home/kali") {
		t.Error("nonexistent session should not be valid")
	}

	newest, ok := c.NewestSession("/home/kali")
	if !ok || newest != "newer-session" {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", newest, ok, "newer-session")
	}
}

func TestCodexMatches(t *testing.T) {
	c := Codex{}
	cases := map[string]bool{
		"codex resume abc":  true,
		"/usr/bin/codex":    true,
		"opencode -s ses_x": false,
	}
	for cmdline, want := range cases {
		if got := c.Matches(cmdline); got != want {
			t.Errorf("Matches(%q) = %v, want %v", cmdline, got, want)
		}
	}
}
