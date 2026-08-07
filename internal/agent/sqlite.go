package agent

import (
	"os/exec"
	"strings"
)

// sqliteQuoteString escapes a value for safe embedding inside a single-quoted
// SQLite string literal. Session IDs and cwds come from data this program
// itself captured, but they still pass through here unescaped otherwise --
// no reason to trust that blindly.
func sqliteQuoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// querySQLiteOne shells out to the sqlite3 CLI (present wherever the agent
// whose DB this is was itself installed -- none of these agents work
// without it) and returns the first column of the first row, if any.
// Deliberately avoids a cgo/pure-Go SQLite driver dependency: this tool
// stays a single static binary, and a missing sqlite3 binary just means
// this one check degrades to "unknown" rather than failing the build.
func querySQLiteOne(dbPath, query string) (string, bool) {
	out, err := exec.Command("sqlite3", "-readonly", dbPath, query).Output()
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if line == "" {
		return "", false
	}
	return line, true
}
