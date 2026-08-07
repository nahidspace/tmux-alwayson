package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHermesValidSession(t *testing.T) {
	requireSQLite3(t)
	home := t.TempDir()
	dbDir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	makeTestDB(t, dbDir, "state.db", `
		CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			cwd TEXT,
			started_at REAL,
			last_activity_at REAL,
			archived INTEGER DEFAULT 0
		);
		INSERT INTO sessions VALUES ('20260807_175151_409a56', '/home/kali', 100, 200, 0);
		INSERT INTO sessions VALUES ('cron_8ffb7be435e1_20260807_202455', '/home/kali', 50, 50, 0);
		INSERT INTO sessions VALUES ('archived_session', '/home/kali', 300, 400, 1);
	`)

	h := Hermes{Home: home}

	if !h.ValidSession("20260807_175151_409a56", "/home/kali") {
		t.Error("real session should be valid")
	}
	if h.ValidSession("no-such-session", "/home/kali") {
		t.Error("nonexistent session should not be valid")
	}

	newest, ok := h.NewestSession("/home/kali")
	if !ok || newest != "20260807_175151_409a56" {
		t.Errorf("NewestSession = (%q, %v), want the most recently active, non-archived session", newest, ok)
	}
}

func TestHermesResumeCommand(t *testing.T) {
	h := Hermes{}
	got := h.ResumeCommand("abc123", []string{"--model", "gpt-5"})
	want := []string{"hermes", "--model", "gpt-5", "--resume", "abc123"}
	if len(got) != len(want) {
		t.Fatalf("ResumeCommand = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ResumeCommand = %v, want %v", got, want)
		}
	}
}
