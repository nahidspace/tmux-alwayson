// Package guard implements the save-time safety net: run the real
// tmux-resurrect save, reject the result if it captured nothing (tmux's own
// backgrounded status-bar job for this is unreliable -- see the project
// README), and repair any captured session ID that turns out to be
// unusable before it ever reaches a restore.
package guard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nahidspace/tmux-alwayson/internal/agent"
)

type Paths struct {
	Home             string
	ResurrectDataDir string // where save.sh writes its layout file + assistant-sessions.json
	SaveScript       string // path to tmux-resurrect's own scripts/save.sh
}

// DefaultPaths resolves the standard locations, mirroring tmux-resurrect's
// own resurrect_data_dir() resolution order: $TMUX_RESURRECT_DIR, the
// @resurrect-dir tmux option, ~/.tmux/resurrect if it exists, else the XDG
// default.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}

	dataDir := os.Getenv("TMUX_RESURRECT_DIR")
	if dataDir == "" {
		if out, err := exec.Command("tmux", "show-option", "-gqv", "@resurrect-dir").Output(); err == nil {
			if v := trimNL(string(out)); v != "" {
				dataDir = expandHome(v, home)
			}
		}
	}
	if dataDir == "" {
		legacy := filepath.Join(home, ".tmux", "resurrect")
		if info, err := os.Stat(legacy); err == nil && info.IsDir() {
			dataDir = legacy
		}
	}
	if dataDir == "" {
		xdg := os.Getenv("XDG_DATA_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".local", "share")
		}
		dataDir = filepath.Join(xdg, "tmux", "resurrect")
	}

	return Paths{
		Home:             home,
		ResurrectDataDir: dataDir,
		SaveScript:       filepath.Join(home, ".tmux", "plugins", "tmux-resurrect", "scripts", "save.sh"),
	}, nil
}

func trimNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func expandHome(p, home string) string {
	if p == "~" || len(p) >= 2 && p[:2] == "~/" {
		return home + p[1:]
	}
	return p
}

// Run executes one guarded save cycle: real save, pane-count check,
// session-ID repair. Returns a human-readable summary line for logging.
func Run(paths Paths, reg *agent.Registry) (string, error) {
	lastLink := filepath.Join(paths.ResurrectDataDir, "last")
	oldTarget, _ := os.Readlink(lastLink) // ok if empty: first-ever save

	cmd := exec.Command("bash", paths.SaveScript, "quiet")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("save.sh failed: %w (%s)", err, string(out))
	}

	newTarget, err := os.Readlink(lastLink)
	if err != nil {
		return "", fmt.Errorf("save.sh ran but left no 'last' symlink: %w", err)
	}

	panes, err := countPaneLines(filepath.Join(paths.ResurrectDataDir, newTarget))
	if err != nil {
		return "", err
	}

	if panes == 0 && oldTarget != "" && oldTarget != newTarget {
		oldPath := filepath.Join(paths.ResurrectDataDir, oldTarget)
		if _, err := os.Stat(oldPath); err == nil {
			if err := atomicRelink(lastLink, oldTarget); err != nil {
				return "", fmt.Errorf("reverting empty save: %w", err)
			}
			return fmt.Sprintf("reverted empty save %q -> kept %q", newTarget, oldTarget), nil
		}
	}

	repaired, err := repairSessions(paths, reg)
	if err != nil {
		// A repair failure shouldn't fail the whole save -- the layout
		// itself is already safely captured above.
		return fmt.Sprintf("save ok (%d panes); session repair skipped: %v", panes, err), nil
	}
	if repaired > 0 {
		return fmt.Sprintf("save ok (%d panes); repaired %d session ID(s)", panes, repaired), nil
	}
	return fmt.Sprintf("save ok (%d panes)", panes), nil
}

func countPaneLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("reading new save file: %w", err)
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) >= 4 && line[:4] == "pane" {
			count++
		}
	}
	return count, scanner.Err()
}

// atomicRelink points link at target via a temp-symlink-then-rename, so a
// crash mid-update can never leave "last" missing or half-written -- unlike
// `ln -fs`, which unlinks and relinks as two separate steps.
func atomicRelink(link, target string) error {
	tmp := link + ".tmp"
	os.Remove(tmp) // best-effort; a leftover from a prior crash shouldn't block us
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, link)
}

type sidecarFile struct {
	Timestamp string         `json:"timestamp"`
	Sessions  []sidecarEntry `json:"sessions"`
}

type sidecarEntry struct {
	Pane    string          `json:"pane"`
	Tool    string          `json:"tool"`
	Session string          `json:"session_id"`
	Cwd     string          `json:"cwd"`
	Pid     string          `json:"pid"`
	Model   string          `json:"model,omitempty"`
	CliArgs string          `json:"cli_args,omitempty"`
	Env     json.RawMessage `json:"env,omitempty"`
}

// repairSessions validates every captured session ID against the Agent
// registered for its "tool" field, replacing anything unusable with the
// newest real session for that pane's cwd. Entries for tools with no
// registered Agent (yet) are left untouched -- this is what lets OpenCode,
// Codex, etc. be added later without this function needing to change.
func repairSessions(paths Paths, reg *agent.Registry) (int, error) {
	sidecarPath := filepath.Join(paths.ResurrectDataDir, "assistant-sessions.json")
	raw, err := os.ReadFile(sidecarPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var sc sidecarFile
	if err := json.Unmarshal(raw, &sc); err != nil {
		return 0, fmt.Errorf("parsing %s: %w", sidecarPath, err)
	}

	repaired := 0
	kept := make([]sidecarEntry, 0, len(sc.Sessions))
	for _, entry := range sc.Sessions {
		a, ok := reg.Get(entry.Tool)
		if !ok {
			kept = append(kept, entry) // unknown tool: not ours to judge
			continue
		}
		if a.ValidSession(entry.Session, entry.Cwd) {
			kept = append(kept, entry)
			continue
		}
		if newID, ok := a.NewestSession(entry.Cwd); ok && newID != entry.Session {
			entry.Session = newID
			kept = append(kept, entry)
			repaired++
			continue
		}
		// No usable session at all for this pane -- drop the entry rather
		// than hand restore a resume command guaranteed to fail.
		repaired++
	}

	if repaired == 0 {
		return 0, nil
	}

	sc.Sessions = kept
	sc.Timestamp = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	out, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return 0, err
	}
	tmp := sidecarPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, sidecarPath); err != nil {
		return 0, err
	}
	return repaired, nil
}
