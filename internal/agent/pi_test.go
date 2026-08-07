package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeJSONLSession writes a minimal two-line session file: a real header
// record followed by a message record. Marshals the header through
// encoding/json rather than string-concatenating it -- a raw cwd value on
// Windows contains backslashes, which are invalid JSON escapes if pasted
// into a literal directly, silently breaking the parse on the read side.
func writeJSONLSession(t *testing.T, path, id, cwd string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	header, err := json.Marshal(jsonlSessionHeader{Type: "session", ID: id, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	content := string(header) + "\n" + `{"type":"message","role":"user"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPiValidSession(t *testing.T) {
	home := t.TempDir()
	sessionDir := filepath.Join(home, ".pi", "agent", "sessions", "--home-kali--")
	writeJSONLSession(t, filepath.Join(sessionDir, "older.jsonl"), "older-id", "/home/kali")
	time.Sleep(10 * time.Millisecond)
	writeJSONLSession(t, filepath.Join(sessionDir, "newer.jsonl"), "newer-id", "/home/kali")

	p := Pi{Home: home}

	if !p.ValidSession("newer-id", "/home/kali") {
		t.Error("real session should be valid")
	}
	if p.ValidSession("newer-id", "/wrong/cwd") {
		t.Error("session under wrong cwd should not be valid")
	}
	if p.ValidSession("no-such-id", "/home/kali") {
		t.Error("nonexistent session should not be valid")
	}

	newest, ok := p.NewestSession("/home/kali")
	if !ok || newest != "newer-id" {
		t.Errorf("NewestSession = (%q, %v), want (%q, true)", newest, ok, "newer-id")
	}
}

func TestPiTitleRecordSkipped(t *testing.T) {
	home := t.TempDir()
	sessionDir := filepath.Join(home, ".pi", "agent", "sessions", "--home-kali--")
	path := filepath.Join(sessionDir, "with-title.jsonl")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"title","title":"some title"}` + "\n" +
		`{"type":"session","id":"has-a-title","cwd":"/home/kali"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := Pi{Home: home}
	if !p.ValidSession("has-a-title", "/home/kali") {
		t.Error("session header on the second line (after a title record) should still be found")
	}
}
