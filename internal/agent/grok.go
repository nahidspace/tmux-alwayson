package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

// Grok implements Agent for xAI's Grok Build CLI/TUI (not the unrelated
// community `grok-cli`). Every live session is recorded in a JSON registry
// at ~/.grok/active_sessions.json as {session_id, pid, cwd, opened_at},
// updated on session open/close -- so validity is a lookup, not a
// heuristic.
type Grok struct {
	Home string
}

var _ Agent = Grok{}

func (Grok) Name() string { return "grok" }

var grokCmdlinePattern = regexp.MustCompile(`(^|/)grok(\s|$)`)

func (Grok) Matches(cmdline string) bool {
	return grokCmdlinePattern.MatchString(cmdline)
}

func (g Grok) home() string {
	if g.Home != "" {
		return g.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

func (g Grok) registryPath() string {
	if v := os.Getenv("GROK_HOME"); v != "" {
		return filepath.Join(v, "active_sessions.json")
	}
	return filepath.Join(g.home(), ".grok", "active_sessions.json")
}

type grokSessionEntry struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	OpenedAt  string `json:"opened_at"`
}

func (g Grok) readRegistry() ([]grokSessionEntry, bool) {
	data, err := os.ReadFile(g.registryPath())
	if err != nil {
		return nil, false
	}
	var entries []grokSessionEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, false
	}
	return entries, true
}

func (g Grok) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	entries, ok := g.readRegistry()
	if !ok {
		return false
	}
	for _, e := range entries {
		if e.SessionID == sessionID && e.Cwd == cwd {
			return true
		}
	}
	return false
}

func (g Grok) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	entries, ok := g.readRegistry()
	if !ok {
		return "", false
	}
	var bestID, bestOpened string
	for _, e := range entries {
		if e.Cwd != cwd {
			continue
		}
		// opened_at is an ISO-8601-ish timestamp; string comparison sorts
		// correctly for that format without needing to parse it.
		if e.OpenedAt > bestOpened {
			bestOpened = e.OpenedAt
			bestID = e.SessionID
		}
	}
	return bestID, bestID != ""
}

// ResumeCommand deliberately ignores extraArgs: resuming reloads the
// session's own model/agent/context from disk, and grok's prompt is a
// positional argument -- replaying captured args risks re-submitting a
// stale prompt into the resumed session. Mirrors the existing bash
// restore script's own handling of this exact case.
func (Grok) ResumeCommand(sessionID string, extraArgs []string) []string {
	return []string{"grok", "--resume", sessionID}
}
