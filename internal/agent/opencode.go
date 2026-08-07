package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// OpenCode implements Agent for OpenCode (https://github.com/opencode-ai/opencode).
//
// OpenCode records sessions in a SQLite database rather than one file per
// session, so validity here means "does a row with this id and directory
// exist" rather than "does a file exist" (contrast with Claude).
type OpenCode struct {
	// Home overrides the user's home directory; empty means os.UserHomeDir().
	Home string
}

var _ Agent = OpenCode{}

func (OpenCode) Name() string { return "opencode" }

var (
	opencodeCmdlinePattern = regexp.MustCompile(`(^|/)opencode(\s|$)`)
	opencodeSessionIDShape = regexp.MustCompile(`^ses_[A-Za-z0-9_]+$`)
)

func (OpenCode) Matches(cmdline string) bool {
	if !opencodeCmdlinePattern.MatchString(cmdline) {
		return false
	}
	// "opencode run ..." is an LSP subprocess OpenCode spawns for itself,
	// not an interactive session -- excluded the same way the bash hooks
	// exclude it.
	return !strings.Contains(cmdline, "opencode run ")
}

func (o OpenCode) home() string {
	if o.Home != "" {
		return o.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

func (o OpenCode) dbPath() string {
	return filepath.Join(o.home(), ".local", "share", "opencode", "opencode.db")
}

func (o OpenCode) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" || !opencodeSessionIDShape.MatchString(sessionID) {
		return false
	}
	if _, err := os.Stat(o.dbPath()); err != nil {
		return false
	}
	query := fmt.Sprintf(
		"SELECT 1 FROM session WHERE id = %s AND directory = %s LIMIT 1;",
		sqliteQuoteString(sessionID), sqliteQuoteString(cwd),
	)
	_, ok := querySQLiteOne(o.dbPath(), query)
	return ok
}

func (o OpenCode) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	if _, err := os.Stat(o.dbPath()); err != nil {
		return "", false
	}
	query := fmt.Sprintf(
		"SELECT id FROM session WHERE directory = %s ORDER BY time_updated DESC LIMIT 1;",
		sqliteQuoteString(cwd),
	)
	return querySQLiteOne(o.dbPath(), query)
}

func (OpenCode) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"opencode"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "-s", sessionID)
	return cmd
}
