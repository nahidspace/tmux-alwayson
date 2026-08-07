package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DisableSystemdUnits stops and disables everything tmux-alwayson enabled,
// then removes the unit files. It does not touch the live tmux server --
// killing it out from under a running agent isn't this command's call to
// make.
func DisableSystemdUnits() error {
	units := []string{"tmux-alwayson-save.timer", "tmux-alwayson-save.service", "tmux.service"}
	for _, u := range units {
		// disable --now on a unit that was never enabled just no-ops with a
		// warning on stderr; not worth treating as fatal.
		exec.Command("systemctl", "--user", "disable", "--now", u).Run()
	}

	p, err := DefaultPaths()
	if err != nil {
		return err
	}
	for _, u := range units {
		path := filepath.Join(p.SystemdUserDir, u)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w (%s)", err, string(out))
	}
	return nil
}

// StripTmuxConf removes tmux-alwayson's marker block from ~/.tmux.conf,
// leaving everything else -- including the TPM init line, and any plugins
// the user added independently of this tool -- exactly as it was.
func StripTmuxConf(p Paths) error {
	data, err := os.ReadFile(p.TmuxConf)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := splitLines(string(data))
	var kept []string
	inBlock := false
	for _, line := range lines {
		switch trimSpace(line) {
		case confBeginMarker:
			inBlock = true
			continue
		case confEndMarker:
			inBlock = false
			continue
		}
		if inBlock {
			continue
		}
		kept = append(kept, line)
	}
	return os.WriteFile(p.TmuxConf, []byte(joinLines(kept)), 0o644)
}

// PurgeClones removes the plugin and hooks-repo checkouts this tool made
// (TPM, tmux-resurrect, tmux-continuum, the agent-hooks repo) and disables
// lingering. It deliberately does NOT touch ~/.claude/settings.json or any
// other agent-side hook registration those checkouts installed -- undoing
// that safely means replaying the hooks repo's own uninstall logic, not
// guessing at JSON surgery from here. Run the hooks repo's own `just
// uninstall` (or remove the relevant hook entries by hand) if you need
// that too.
func PurgeClones(p Paths) error {
	for _, dir := range []string{p.HooksRepoDir, p.TPMDir, p.ResurrectDir, p.ContinuumDir} {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("removing %s: %w", dir, err)
		}
	}

	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	exec.Command("loginctl", "disable-linger", user).Run() // best-effort
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
