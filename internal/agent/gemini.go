package agent

import (
	"os"
	"path/filepath"
	"regexp"
)

// Gemini implements Agent for Google's Gemini CLI
// (github.com/google-gemini/gemini-cli). Sessions are recorded as JSONL
// files under ~/.gemini/tmp/<sha256-hex-of-project-root>/chats/, one file
// per session, with a metadata record (sessionId, projectHash, ...) as the
// first line.
//
// This deliberately does not try to recompute the project-hash directory
// name itself (sha256 of an exact, unnormalized project root string is
// easy to get subtly wrong -- trailing slash, symlink resolution, etc. --
// and getting it wrong silently means "session not found" instead of an
// error). Instead it scans every project directory under ~/.gemini/tmp and
// matches by the sessionId recorded inside each file, which is
// unambiguous regardless of how the directory got named.
//
// Implemented from Gemini CLI's public source and docs, not verified
// against a live install -- unlike Claude, which was.
type Gemini struct {
	Home string
}

var _ Agent = Gemini{}

func (Gemini) Name() string { return "gemini" }

var geminiCmdlinePattern = regexp.MustCompile(`(^|/)gemini(\s|$)`)

func (Gemini) Matches(cmdline string) bool {
	return geminiCmdlinePattern.MatchString(cmdline)
}

func (g Gemini) home() string {
	if g.Home != "" {
		return g.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

func (g Gemini) projectsRoot() string {
	return filepath.Join(g.home(), ".gemini", "tmp")
}

func (g Gemini) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	return scanProjectsForSession(g.projectsRoot(), "chats", sessionID, cwd)
}

func (g Gemini) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	return newestSessionAcrossProjects(g.projectsRoot(), "chats", cwd)
}

func (Gemini) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"gemini"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "--resume", sessionID)
	return cmd
}
