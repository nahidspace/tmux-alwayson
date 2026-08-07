package agent

import (
	"os"
	"path/filepath"
	"regexp"
)

// Pi implements Agent for the Pi coding agent (github.com/mariozechner/pi).
// Sessions are one JSONL file per session under
// ~/.pi/agent/sessions/--<sanitized-cwd>--/, each starting with a
// {"type":"session","id":...,"cwd":...} header record.
type Pi struct {
	Home string
}

var _ Agent = Pi{}

func (Pi) Name() string { return "pi" }

var piCmdlinePattern = regexp.MustCompile(`(^|/)pi(\s|$)`)

func (Pi) Matches(cmdline string) bool {
	return piCmdlinePattern.MatchString(cmdline)
}

func (p Pi) home() string {
	if p.Home != "" {
		return p.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

func (p Pi) sessionsRoot() string {
	if dir := os.Getenv("PI_CODING_AGENT_SESSION_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(p.home(), ".pi", "agent", "sessions")
}

func (p Pi) sessionDirsFor(cwd string) []string {
	return []string{filepath.Join(p.sessionsRoot(), "--"+sanitizePathComponent(cwd)+"--")}
}

func (p Pi) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	return jsonlSessionExists(p.sessionDirsFor(cwd), sessionID, cwd)
}

func (p Pi) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	return newestJSONLSession(p.sessionDirsFor(cwd), cwd)
}

func (Pi) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"pi"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "--session", sessionID)
	return cmd
}
