package agent

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// requireSQLite3 skips the test if the sqlite3 CLI isn't on PATH -- these
// agents don't work without it either, so a missing binary here just means
// "can't verify this locally," not a real failure.
func requireSQLite3(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
}

// makeTestDB creates a fresh SQLite file at dir/name and runs schemaSQL
// against it via the sqlite3 CLI.
func makeTestDB(t *testing.T, dir, name, schemaSQL string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	cmd := exec.Command("sqlite3", path, schemaSQL)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("creating test db: %v (%s)", err, out)
	}
	return path
}
