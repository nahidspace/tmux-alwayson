package agent

import (
	"path/filepath"
	"testing"
)

func TestQwenValidSession(t *testing.T) {
	home := t.TempDir()
	chats := filepath.Join(home, ".qwen", "projects", "my-project", "chats")
	writeGenericSession(t, filepath.Join(chats, "session.jsonl"), genericSessionMeta{
		SessionID: "qwen-real-session-id",
		Cwd:       "/home/kali/project",
		Mtime:     "2026-08-07T10:00:00Z",
	})

	q := Qwen{Home: home}

	if !q.ValidSession("qwen-real-session-id", "/home/kali/project") {
		t.Error("real session should be valid")
	}
	if q.ValidSession("qwen-real-session-id", "/wrong/cwd") {
		t.Error("session under wrong cwd should not be valid")
	}
	if q.ValidSession("no-such-session", "/home/kali/project") {
		t.Error("nonexistent session should not be valid")
	}
}

func TestQwenMatches(t *testing.T) {
	q := Qwen{}
	cases := map[string]bool{
		"qwen --resume abc": true,
		"/usr/bin/qwen":     true,
		"gemini --resume x": false,
	}
	for cmdline, want := range cases {
		if got := q.Matches(cmdline); got != want {
			t.Errorf("Matches(%q) = %v, want %v", cmdline, got, want)
		}
	}
}
