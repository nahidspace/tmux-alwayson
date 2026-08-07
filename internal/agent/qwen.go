package agent

import (
	"os"
	"path/filepath"
	"regexp"
)

// Qwen implements Agent for Alibaba's Qwen Code CLI
// (github.com/QwenLM/qwen-code), a fork of Gemini CLI's architecture.
// Sessions are recorded as JSONL files under
// ~/.qwen/projects/<project-key>/chats/, with a metadata record including
// sessionId and cwd as the first line -- same shape as Gemini CLI, so this
// reuses the same project-directory scan rather than the checkpoint/shadow
// git storage under ~/.qwen/history (a different feature: file-change
// checkpoints, not session resume).
//
// Implemented from Qwen Code's public docs and issue reports, not verified
// against a live install.
type Qwen struct {
	Home string
}

var _ Agent = Qwen{}

func (Qwen) Name() string { return "qwen" }

var qwenCmdlinePattern = regexp.MustCompile(`(^|/)qwen(\s|$)`)

func (Qwen) Matches(cmdline string) bool {
	return qwenCmdlinePattern.MatchString(cmdline)
}

func (q Qwen) home() string {
	if q.Home != "" {
		return q.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

func (q Qwen) projectsRoot() string {
	return filepath.Join(q.home(), ".qwen", "projects")
}

func (q Qwen) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	return scanProjectsForSession(q.projectsRoot(), "chats", sessionID, cwd)
}

func (q Qwen) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	return newestSessionAcrossProjects(q.projectsRoot(), "chats", cwd)
}

func (Qwen) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"qwen"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "--resume", sessionID)
	return cmd
}
