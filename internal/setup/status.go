package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type checkResult struct {
	ok   bool
	name string
	note string
}

func Status(p Paths) string {
	var results []checkResult

	results = append(results, dirCheck("TPM", p.TPMDir))
	results = append(results, dirCheck("tmux-resurrect", p.ResurrectDir))
	results = append(results, dirCheck("tmux-continuum", p.ContinuumDir))
	results = append(results, dirCheck("agent hooks repo", p.HooksRepoDir))
	results = append(results, tmuxConfCheck(p.TmuxConf))
	results = append(results, systemdUnitCheck("tmux.service"))
	results = append(results, systemdTimerCheck("tmux-alwayson-save.timer"))
	results = append(results, lingerCheck())

	var b strings.Builder
	b.WriteString("=== tmux-alwayson status ===\n\n")
	for _, r := range results {
		mark := "[ok]"
		if !r.ok {
			mark = "[!!]"
		}
		if r.note != "" {
			fmt.Fprintf(&b, "%s %s (%s)\n", mark, r.name, r.note)
		} else {
			fmt.Fprintf(&b, "%s %s\n", mark, r.name)
		}
	}
	return b.String()
}

func dirCheck(name, dir string) checkResult {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return checkResult{false, name, "not found at " + dir}
	}
	return checkResult{true, name, ""}
}

func tmuxConfCheck(path string) checkResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return checkResult{false, "tmux.conf configured", "cannot read " + path}
	}
	if strings.Contains(string(data), confBeginMarker) {
		return checkResult{true, "tmux.conf configured", ""}
	}
	return checkResult{false, "tmux.conf configured", "run 'tmux-alwayson install'"}
}

func systemdUnitCheck(unit string) checkResult {
	out, err := exec.Command("systemctl", "--user", "is-enabled", unit).Output()
	state := strings.TrimSpace(string(out))
	if err != nil {
		return checkResult{false, unit, "not enabled (" + state + ")"}
	}
	activeOut, _ := exec.Command("systemctl", "--user", "is-active", unit).Output()
	return checkResult{true, unit, "enabled, " + strings.TrimSpace(string(activeOut))}
}

func systemdTimerCheck(timer string) checkResult {
	res := systemdUnitCheck(timer)
	if !res.ok {
		return res
	}
	out, _ := exec.Command("systemctl", "--user", "list-timers", timer, "--no-pager").Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) >= 2 {
		res.note = strings.TrimSpace(lines[1])
	}
	return res
}

func lingerCheck() checkResult {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	out, err := exec.Command("loginctl", "show-user", user, "-p", "Linger").Output()
	if err != nil {
		return checkResult{false, "loginctl linger", "could not query"}
	}
	if strings.TrimSpace(string(out)) == "Linger=yes" {
		return checkResult{true, "loginctl linger", ""}
	}
	return checkResult{false, "loginctl linger", "run: loginctl enable-linger " + user}
}

// LastSaveSummary reads the resurrect data dir's "last" pointer and reports
// what it currently contains, without needing jq.
func LastSaveSummary(resurrectDataDir string) string {
	lastLink := filepath.Join(resurrectDataDir, "last")
	target, err := os.Readlink(lastLink)
	if err != nil {
		return "no saved session yet"
	}
	info, err := os.Stat(filepath.Join(resurrectDataDir, target))
	if err != nil {
		return fmt.Sprintf("last save %q is missing on disk", target)
	}
	return fmt.Sprintf("last save: %s (%d bytes, modified %s)", target, info.Size(), info.ModTime().Format("2006-01-02 15:04:05"))
}
