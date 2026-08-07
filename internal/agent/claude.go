package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Claude implements Agent for Claude Code.
//
// Claude's SessionStart hook writes a session_id the instant the process
// launches, before any real turn has happened -- the transcript .jsonl
// isn't written until the first reply completes. A process killed in that
// window (a rapid restart, an abandoned launch, a crash) leaves a
// session_id that looks completely valid but backs a conversation that
// never existed. --resume on that ID fails outright ("No conversation
// found..."), dropping the pane to a bare shell. ValidSession exists
// specifically to catch that before it's trusted.
type Claude struct {
	// Home overrides the user's home directory; empty means os.UserHomeDir().
	// Exists for tests.
	Home string
}

var _ Agent = Claude{}

func (Claude) Name() string { return "claude" }

var claudeCmdlinePattern = regexp.MustCompile(`(^|/)claude(\s|$)`)

func (Claude) Matches(cmdline string) bool {
	return claudeCmdlinePattern.MatchString(cmdline)
}

func (c Claude) home() string {
	if c.Home != "" {
		return c.Home
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// projectDir mirrors Claude Code's own layout convention: the cwd with
// every non-alphanumeric character replaced by a dash, prefixed with one
// more dash from the leading path separator. E.g. /home/kali -> -home-kali.
func (c Claude) projectDir(cwd string) string {
	safe := sanitizeCwd(cwd)
	return filepath.Join(c.home(), ".claude", "projects", safe)
}

func sanitizeCwd(cwd string) string {
	trimmed := strings.TrimPrefix(cwd, "/")
	var b strings.Builder
	b.WriteByte('-')
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

func (c Claude) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	path := filepath.Join(c.projectDir(cwd), sessionID+".jsonl")
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

func (c Claude) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	dir := c.projectDir(cwd)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	type candidate struct {
		id      string
		modTime int64
	}
	var candidates []candidate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		candidates = append(candidates, candidate{
			id:      strings.TrimSuffix(e.Name(), ".jsonl"),
			modTime: info.ModTime().UnixNano(),
		})
	}
	if len(candidates) == 0 {
		return "", false
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modTime > candidates[j].modTime })
	return candidates[0].id, true
}

func (Claude) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"claude"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "--resume", sessionID)
	return cmd
}
