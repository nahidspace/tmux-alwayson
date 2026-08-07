package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeGenericSession(t *testing.T, path string, m genericSessionMeta) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data) + "\n" + `{"type":"user","content":"hi"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGeminiValidSession(t *testing.T) {
	home := t.TempDir()
	// Directory name is deliberately NOT a real sha256 hash here -- Gemini
	// matches by scanning file content, not by recomputing this name, so
	// an arbitrary project-dir name must still work.
	chats := filepath.Join(home, ".gemini", "tmp", "somehash123", "chats")
	writeGenericSession(t, filepath.Join(chats, "2026-08-07T10-00-abcd1234.jsonl"), genericSessionMeta{
		SessionID:   "abcd1234-real-full-session-id",
		Cwd:         "/home/kali",
		LastUpdated: "2026-08-07T10:00:00Z",
	})

	g := Gemini{Home: home}

	if !g.ValidSession("abcd1234-real-full-session-id", "/home/kali") {
		t.Error("real session should be valid, found by scanning content not by directory name")
	}
	if g.ValidSession("abcd1234-real-full-session-id", "/wrong/cwd") {
		t.Error("session under wrong cwd should not be valid")
	}
	if g.ValidSession("no-such-session", "/home/kali") {
		t.Error("nonexistent session should not be valid")
	}
}

func TestGeminiNewestSessionAcrossMultipleProjectDirs(t *testing.T) {
	home := t.TempDir()
	writeGenericSession(t,
		filepath.Join(home, ".gemini", "tmp", "hashA", "chats", "older.jsonl"),
		genericSessionMeta{SessionID: "older-id", Cwd: "/home/kali", LastUpdated: "2026-08-07T09:00:00Z"})
	writeGenericSession(t,
		filepath.Join(home, ".gemini", "tmp", "hashB", "chats", "newer.jsonl"),
		genericSessionMeta{SessionID: "newer-id", Cwd: "/home/kali", LastUpdated: "2026-08-07T11:00:00Z"})

	g := Gemini{Home: home}
	newest, ok := g.NewestSession("/home/kali")
	if !ok || newest != "newer-id" {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", newest, ok, "newer-id")
	}
}
