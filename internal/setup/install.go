package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RequireBinaries checks that every external tool tmux-alwayson depends on
// (and does not attempt to install itself -- installing packages is the
// user's call, not this tool's) is on PATH.
func RequireBinaries(names ...string) error {
	var missing []string
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required commands: %s (install them first)", strings.Join(missing, ", "))
	}
	return nil
}

func EnsureCloned(url, dir string) (cloned bool, err error) {
	if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return false, err
	}
	cmd := exec.Command("git", "clone", "--depth", "1", url, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git clone %s: %w (%s)", url, err, string(out))
	}
	return true, nil
}

const (
	confBeginMarker = "# --- begin tmux-alwayson ---"
	confEndMarker   = "# --- end tmux-alwayson ---"
)

// PatchTmuxConf writes (or rewrites, idempotently) the config block that
// wires resurrect/continuum options and points the save/restore hooks at
// hooksRepoDir. Safe to call repeatedly -- it replaces its own prior block
// by marker, and preserves everything else in the file untouched, moving
// the TPM init line to stay last (tmux requires it).
func PatchTmuxConf(p Paths) error {
	existing, err := os.ReadFile(p.TmuxConf)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(existing), "\n")

	var kept []string
	var tpmInitLine string
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == confBeginMarker:
			inBlock = true
			continue
		case trimmed == confEndMarker:
			inBlock = false
			continue
		case inBlock:
			continue
		case strings.Contains(line, "tpm/tpm") && !strings.HasPrefix(trimmed, "#"):
			tpmInitLine = line
			continue
		default:
			kept = append(kept, line)
		}
	}

	// Trim trailing blank lines so the block starts cleanly.
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}

	block := []string{
		"",
		confBeginMarker,
		"set -g @plugin 'tmux-plugins/tpm'",
		"set -g @plugin 'tmux-plugins/tmux-resurrect'",
		"set -g @plugin 'tmux-plugins/tmux-continuum'",
		fmt.Sprintf("set -g @resurrect-hook-post-save-all \"bash '%s'\"", filepath.Join(p.HooksRepoDir, "scripts", "save-assistant-sessions.sh")),
		fmt.Sprintf("set -g @resurrect-hook-post-restore-all \"bash '%s'\"", filepath.Join(p.HooksRepoDir, "scripts", "restore-assistant-sessions.sh")),
		"set -g @continuum-restore 'on'",
		"set -g @continuum-boot 'on'",
		// tmux-alwayson's own systemd timer drives periodic saves through
		// the Go-native guard -- tmux-continuum's own interval must stay
		// off, or its unreliable backgrounded status-bar job races it.
		"set -g @continuum-save-interval '0'",
		confEndMarker,
	}
	kept = append(kept, block...)

	if tpmInitLine == "" {
		tpmInitLine = fmt.Sprintf("run '%s'", filepath.Join(p.TPMDir, "tpm"))
	}
	kept = append(kept, tpmInitLine)

	return os.WriteFile(p.TmuxConf, []byte(strings.Join(kept, "\n")+"\n"), 0o644)
}

// tmpTmuxSession runs fn with a scratch detached tmux session guaranteed to
// exist for its duration -- several install steps (TPM's install_plugins,
// the hooks repo's own .tmux entry point) need a live server with the
// current config sourced, and must not disturb any session the user
// already has running.
func tmpTmuxSession(fn func() error) error {
	const name = "__tmux_alwayson_install__"
	started := false
	if err := exec.Command("tmux", "has-session", "-t", name).Run(); err != nil {
		if out, err := exec.Command("tmux", "new-session", "-d", "-s", name).CombinedOutput(); err != nil {
			// A totally absent server also satisfies "no session named
			// this exists" via has-session's exit code, so this creates it.
			return fmt.Errorf("starting scratch tmux session: %w (%s)", err, string(out))
		}
		started = true
	}
	defer func() {
		if started {
			exec.Command("tmux", "kill-session", "-t", name).Run()
		}
	}()

	if out, err := exec.Command("tmux", "send-keys", "-t", name, "tmux source-file ~/.tmux.conf", "Enter").CombinedOutput(); err != nil {
		return fmt.Errorf("sourcing tmux.conf: %w (%s)", err, string(out))
	}
	if err := waitForTPMEnv(); err != nil {
		return err
	}
	return fn()
}

// waitForTPMEnv blocks until TPM's `run '.../tpm'` line (triggered by the
// source-file above) has actually executed and set TMUX_PLUGIN_MANAGER_PATH
// in the server's global environment. source-file's `run` directive launches
// that script in the background, so without this wait install_plugins can
// start before the variable exists, failing with "unknown variable:
// TMUX_PLUGIN_MANAGER_PATH".
func waitForTPMEnv() error {
	const timeout = 5 * time.Second
	const interval = 100 * time.Millisecond
	deadline := time.Now().Add(timeout)
	for {
		if exec.Command("tmux", "show-environment", "-g", "TMUX_PLUGIN_MANAGER_PATH").Run() == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for tpm to set TMUX_PLUGIN_MANAGER_PATH (is the tpm run line present in tmux.conf?)", timeout)
		}
		time.Sleep(interval)
	}
}

func InstallTPMPlugins(p Paths) error {
	return tmpTmuxSession(func() error {
		installScript := filepath.Join(p.TPMDir, "bin", "install_plugins")
		out, err := exec.Command(installScript).CombinedOutput()
		if err != nil {
			return fmt.Errorf("tpm install_plugins: %w (%s)", err, string(out))
		}
		return nil
	})
}

// InstallHooks runs the hooks repo's own TPM entry point (installs the
// Claude SessionStart/SessionEnd hooks and the OpenCode plugin) -- the
// exact same delegation the repo's own `just install-claude-hook` uses.
func InstallHooks(p Paths) error {
	return tmpTmuxSession(func() error {
		entry := filepath.Join(p.HooksRepoDir, "tmux-assistant-resurrect.tmux")
		out, err := exec.Command("bash", entry).CombinedOutput()
		if err != nil {
			return fmt.Errorf("installing agent hooks: %w (%s)", err, string(out))
		}
		return nil
	})
}

// InstallSelf copies the running binary to SelfBinDir so systemd units have
// a stable absolute path to invoke, independent of wherever the user first
// ran `tmux-alwayson install` from. Returns that stable path.
func InstallSelf(p Paths) (string, error) {
	src, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(p.SelfBinDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(p.SelfBinDir, "tmux-alwayson")

	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return "", err
	}
	return dst, nil
}

func EnableLinger() error {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	out, err := exec.Command("loginctl", "enable-linger", user).CombinedOutput()
	if err != nil {
		return fmt.Errorf("loginctl enable-linger: %w (%s)", err, string(out))
	}
	return nil
}
