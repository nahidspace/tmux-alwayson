package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Hermes implements Agent for the Hermes Agent CLI/TUI
// (github.com/nahidspace/hermes-agent-upstream) -- a Claude-Code-shaped
// interactive assistant that is separate from its own messaging-gateway
// daemon mode (`hermes gateway run`). Sessions live as rows in a SQLite
// store at ~/.hermes/state.db.
//
// Detection of Hermes panes by the shared bash hooks in
// tmux-assistant-resurrect does not exist yet -- Matches/ValidSession/
// NewestSession here are ready to be wired in (either by extending those
// hooks, or by having tmux-alwayson do its own process scan via Matches),
// but no sidecar entry with "tool": "hermes" will be produced until one of
// those exists. This implementation is the agent-side half of that work.
type Hermes struct {
	Home string
}

var _ Agent = Hermes{}

func (Hermes) Name() string { return "hermes" }

var hermesCmdlinePattern = regexp.MustCompile(`(^|/)hermes(\s|$)`)

func (Hermes) Matches(cmdline string) bool {
	return hermesCmdlinePattern.MatchString(cmdline)
}

func (h Hermes) home() string {
	if h.Home != "" {
		return h.Home
	}
	hd, _ := os.UserHomeDir()
	return hd
}

func (h Hermes) dbPath() string {
	return filepath.Join(h.home(), ".hermes", "state.db")
}

func (h Hermes) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	if _, err := os.Stat(h.dbPath()); err != nil {
		return false
	}
	query := fmt.Sprintf(
		"SELECT 1 FROM sessions WHERE id = %s AND cwd = %s LIMIT 1;",
		sqliteQuoteString(sessionID), sqliteQuoteString(cwd),
	)
	_, ok := querySQLiteOne(h.dbPath(), query)
	return ok
}

func (h Hermes) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	if _, err := os.Stat(h.dbPath()); err != nil {
		return "", false
	}
	query := fmt.Sprintf(
		"SELECT id FROM sessions WHERE cwd = %s AND (archived IS NULL OR archived = 0) "+
			"ORDER BY COALESCE(last_activity_at, started_at) DESC LIMIT 1;",
		sqliteQuoteString(cwd),
	)
	return querySQLiteOne(h.dbPath(), query)
}

func (Hermes) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"hermes"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "--resume", sessionID)
	return cmd
}
