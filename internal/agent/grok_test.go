package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGrokRegistry(t *testing.T, path string, entries []grokSessionEntry) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `[`
	for i, e := range entries {
		if i > 0 {
			data += ","
		}
		data += `{"session_id":"` + e.SessionID + `","cwd":"` + e.Cwd + `","opened_at":"` + e.OpenedAt + `"}`
	}
	data += `]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGrokValidSession(t *testing.T) {
	home := t.TempDir()
	writeGrokRegistry(t, filepath.Join(home, ".grok", "active_sessions.json"), []grokSessionEntry{
		{SessionID: "older-uuid", Cwd: "/home/kali", OpenedAt: "2026-08-07T10:00:00Z"},
		{SessionID: "newer-uuid", Cwd: "/home/kali", OpenedAt: "2026-08-07T12:00:00Z"},
		{SessionID: "other-cwd-uuid", Cwd: "/somewhere/else", OpenedAt: "2026-08-07T13:00:00Z"},
	})

	g := Grok{Home: home}

	if !g.ValidSession("newer-uuid", "/home/kali") {
		t.Error("real session should be valid")
	}
	if g.ValidSession("newer-uuid", "/wrong/cwd") {
		t.Error("session under wrong cwd should not be valid")
	}
	if g.ValidSession("closed-uuid", "/home/kali") {
		t.Error("session not in the registry should not be valid")
	}

	newest, ok := g.NewestSession("/home/kali")
	if !ok || newest != "newer-uuid" {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", newest, ok, "newer-uuid")
	}
}

func TestGrokResumeCommandIgnoresExtraArgs(t *testing.T) {
	g := Grok{}
	got := g.ResumeCommand("abc-123", []string{"some", "stale", "prompt", "args"})
	want := []string{"grok", "--resume", "abc-123"}
	if len(got) != len(want) {
		t.Fatalf("ResumeCommand = %v, want %v (extraArgs must be ignored)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ResumeCommand = %v, want %v", got, want)
		}
	}
}
