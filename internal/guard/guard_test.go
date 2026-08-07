package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nahidspace/tmux-alwayson/internal/agent"
)

// countPaneLines is the exact check that replaced `grep -c '^pane' file ||
// echo 0` in the original bash guard -- that fallback silently broke
// because grep exits 1 on zero matches even though it printed "0"
// correctly. These cases pin the behavior that bug produced wrong results
// for.
func TestCountPaneLines(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"empty file", "", 0},
		{"no pane lines", "window\tdemo\t0\nstate\t\t\n", 0},
		{"one pane", "pane\tdemo\t0\t1\n", 1},
		{"multiple panes and other lines", "pane\ta\t0\t1\nwindow\ta\t0\npane\ta\t0\t2\nstate\t\t\n", 2},
		{"pane-like prefix that isn't the line type", "panecake\tnot\treal\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "save.txt")
			if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := countPaneLines(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("countPaneLines(%q) = %d, want %d", c.content, got, c.want)
			}
		})
	}
}

func TestAtomicRelink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "last")

	if err := os.WriteFile(filepath.Join(dir, "good.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("good.txt", link); err != nil {
		if os.IsPermission(err) || strings.Contains(err.Error(), "privilege") {
			t.Skip("creating symlinks requires elevated privileges on this platform (e.g. Windows without Developer Mode)")
		}
		t.Fatal(err)
	}

	if err := atomicRelink(link, "good.txt"); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(link)
	if err != nil || target != "good.txt" {
		t.Errorf("Readlink = (%q, %v), want (%q, nil)", target, err, "good.txt")
	}

	// A leftover .tmp from a prior crash must not block a fresh relink.
	if err := os.Symlink("stale", link+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := atomicRelink(link, "good.txt"); err != nil {
		t.Fatalf("relink with a stale .tmp present should still succeed: %v", err)
	}
}

// stubAgent is a minimal Agent for exercising repairSessions without
// needing a real Claude/OpenCode/etc install.
type stubAgent struct {
	name   string
	valid  map[string]bool   // "sessionID|cwd" -> valid
	newest map[string]string // cwd -> newest session id (empty means none)
}

func (s stubAgent) Name() string        { return s.name }
func (s stubAgent) Matches(string) bool { return false }
func (s stubAgent) ValidSession(id, cwd string) bool {
	return s.valid[id+"|"+cwd]
}
func (s stubAgent) NewestSession(cwd string) (string, bool) {
	id, ok := s.newest[cwd]
	return id, ok && id != ""
}
func (stubAgent) ResumeCommand(id string, extra []string) []string { return nil }

var _ agent.Agent = stubAgent{}

func writeSidecar(t *testing.T, path string, sc sidecarFile) {
	t.Helper()
	data, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRepairSessionsReplacesStaleID(t *testing.T) {
	dir := t.TempDir()
	sidecarPath := filepath.Join(dir, "assistant-sessions.json")
	writeSidecar(t, sidecarPath, sidecarFile{
		Sessions: []sidecarEntry{
			{Pane: "demo:0.0", Tool: "claude", Session: "phantom-id", Cwd: "/home/kali"},
		},
	})

	reg := agent.NewRegistry()
	reg.Register(stubAgent{
		name:   "claude",
		valid:  map[string]bool{}, // phantom-id is valid nowhere
		newest: map[string]string{"/home/kali": "real-id"},
	})

	n, err := repairSessions(Paths{ResurrectDataDir: dir}, reg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("repaired count = %d, want 1", n)
	}

	raw, _ := os.ReadFile(sidecarPath)
	var sc sidecarFile
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatal(err)
	}
	if len(sc.Sessions) != 1 || sc.Sessions[0].Session != "real-id" {
		t.Errorf("sessions = %+v, want a single entry with session_id real-id", sc.Sessions)
	}
}

func TestRepairSessionsDropsUnrecoverableEntry(t *testing.T) {
	dir := t.TempDir()
	sidecarPath := filepath.Join(dir, "assistant-sessions.json")
	writeSidecar(t, sidecarPath, sidecarFile{
		Sessions: []sidecarEntry{
			{Pane: "demo:0.0", Tool: "claude", Session: "phantom-id", Cwd: "/home/kali"},
		},
	})

	reg := agent.NewRegistry()
	reg.Register(stubAgent{
		name:   "claude",
		valid:  map[string]bool{},
		newest: map[string]string{}, // nothing usable at all for this cwd
	})

	n, err := repairSessions(Paths{ResurrectDataDir: dir}, reg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("repaired count = %d, want 1", n)
	}

	raw, _ := os.ReadFile(sidecarPath)
	var sc sidecarFile
	json.Unmarshal(raw, &sc)
	if len(sc.Sessions) != 0 {
		t.Errorf("sessions = %+v, want the unrecoverable entry dropped", sc.Sessions)
	}
}

func TestRepairSessionsLeavesValidEntryAndUnknownToolAlone(t *testing.T) {
	dir := t.TempDir()
	sidecarPath := filepath.Join(dir, "assistant-sessions.json")
	writeSidecar(t, sidecarPath, sidecarFile{
		Sessions: []sidecarEntry{
			{Pane: "demo:0.0", Tool: "claude", Session: "good-id", Cwd: "/home/kali"},
			{Pane: "other:0.0", Tool: "some-future-agent", Session: "whatever", Cwd: "/home/kali"},
		},
	})

	reg := agent.NewRegistry()
	reg.Register(stubAgent{
		name:  "claude",
		valid: map[string]bool{"good-id|/home/kali": true},
	})
	// deliberately not registering "some-future-agent" -- an entry for a
	// tool this build doesn't know how to judge must pass through
	// untouched, not get dropped.

	n, err := repairSessions(Paths{ResurrectDataDir: dir}, reg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("repaired count = %d, want 0 (nothing needed fixing)", n)
	}

	raw, _ := os.ReadFile(sidecarPath)
	var sc sidecarFile
	json.Unmarshal(raw, &sc)
	if len(sc.Sessions) != 2 {
		t.Fatalf("sessions = %+v, want both entries untouched", sc.Sessions)
	}
}
