package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// OMP implements Agent for Oh My Pi (github.com/can1357/oh-my-pi).
//
// Oh My Pi's *primary* session lookup in the bash hooks is a per-terminal
// breadcrumb file keyed by pane TTY, plus profile-scoped session roots --
// neither fits the (sessionID, cwd) shape this interface works with (no
// pane/TTY context is available here). This implementation covers the
// fallback that does fit: the default (no-profile) session directory,
// scanned the same way Pi's sessions are. A profile-aware, breadcrumb-aware
// version can be added later without changing the interface.
type OMP struct {
	Home string
}

var _ Agent = OMP{}

func (OMP) Name() string { return "omp" }

var ompCmdlinePattern = regexp.MustCompile(`(^|/)omp(\s|$)`)

func (OMP) Matches(cmdline string) bool {
	if !ompCmdlinePattern.MatchString(cmdline) {
		return false
	}
	return !strings.Contains(cmdline, "__omp_worker_")
}

func (o OMP) home() string {
	if o.Home != "" {
		return o.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

func (o OMP) xdgDataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	return filepath.Join(o.home(), ".local", "share")
}

// sessionDirsFor returns every directory the default (no-profile) OMP
// install might have written cwd's sessions under: the modern
// home/tmp-relative shorthand, the legacy always-full-path form, and the
// pre-XDG fallback under ~/.omp/agent/sessions -- mirroring
// omp_session_dir_names()'s primary+legacy union in the bash hooks.
func (o OMP) sessionDirsFor(cwd string) []string {
	home := strings.TrimRight(o.home(), "/")
	tmp := strings.TrimRight(os.TempDir(), "/")

	var name string
	switch {
	case cwd == home:
		name = "-"
	case strings.HasPrefix(cwd, home+"/"):
		name = "-" + sanitizePathComponent(strings.TrimPrefix(cwd, home+"/"))
	case cwd == tmp:
		name = "-tmp"
	case strings.HasPrefix(cwd, tmp+"/"):
		name = "-tmp-" + sanitizePathComponent(strings.TrimPrefix(cwd, tmp+"/"))
	default:
		name = "--" + sanitizePathComponent(cwd) + "--"
	}
	legacy := "--" + sanitizePathComponent(cwd) + "--"

	roots := []string{filepath.Join(o.xdgDataHome(), "omp", "sessions")}
	if _, err := os.Stat(roots[0]); err != nil {
		roots = []string{filepath.Join(o.home(), ".omp", "agent", "sessions")}
	}

	dirs := []string{filepath.Join(roots[0], name)}
	if legacy != name {
		dirs = append(dirs, filepath.Join(roots[0], legacy))
	}
	return dirs
}

func (o OMP) ValidSession(sessionID, cwd string) bool {
	if sessionID == "" || cwd == "" {
		return false
	}
	return jsonlSessionExists(o.sessionDirsFor(cwd), sessionID, cwd)
}

func (o OMP) NewestSession(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	return newestJSONLSession(o.sessionDirsFor(cwd), cwd)
}

func (OMP) ResumeCommand(sessionID string, extraArgs []string) []string {
	cmd := []string{"omp"}
	cmd = append(cmd, extraArgs...)
	cmd = append(cmd, "--resume", sessionID)
	return cmd
}
