package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// WriteSystemdUnits writes the three units that make sessions survive a
// reboot with zero manual steps:
//
//   - tmux.service: starts tmux on boot with a placeholder session that is
//     NOT named "0" -- tmux-resurrect's own restore cleanup unconditionally
//     kills a session literally named "0" once it's done restoring the
//     real ones, and if "0" is the *only* session that exists (exactly
//     what a bare `tmux new-session -d` produces), that kills the whole
//     server seconds after boot. Type=oneshot + RemainAfterExit=yes avoids
//     a second, unrelated failure mode: systemd's PID-guessing for
//     Type=forking services is flaky under the CPU contention of a real
//     boot and can tear the unit down while tmux is still healthy.
//   - tmux-alwayson-save.timer/.service: periodic saves via this binary's
//     own `guarded-save`, run directly and synchronously by systemd --
//     not through tmux-continuum's own periodic autosave, which fires as a
//     backgrounded job off a status-bar interpolation and is not
//     guaranteed to run to completion.
func WriteSystemdUnits(p Paths, selfPath string) error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("finding tmux binary: %w", err)
	}

	if err := os.MkdirAll(p.SystemdUserDir, 0o755); err != nil {
		return err
	}

	tmuxService := fmt.Sprintf(`[Unit]
Description=tmux default session (detached), managed by tmux-alwayson
Documentation=man:tmux(1)

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=%s new-session -d -s __boot__
ExecStop=%s guarded-save
ExecStop=%s kill-server
KillMode=control-group
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
`, tmuxPath, selfPath, tmuxPath)

	saveService := fmt.Sprintf(`[Unit]
Description=Periodic tmux-alwayson guarded save

[Service]
Type=oneshot
ExecStart=%s guarded-save
`, selfPath)

	saveTimer := `[Unit]
Description=Run tmux-alwayson-save.service every 5 minutes

[Timer]
OnActiveSec=5min
OnUnitActiveSec=5min
AccuracySec=10s

[Install]
WantedBy=timers.target
`

	files := map[string]string{
		"tmux.service":               tmuxService,
		"tmux-alwayson-save.service": saveService,
		"tmux-alwayson-save.timer":   saveTimer,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(p.SystemdUserDir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}

func EnableSystemdUnits() error {
	steps := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "tmux-alwayson-save.timer"},
		{"systemctl", "--user", "enable", "tmux.service"},
	}
	for _, args := range steps {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %w (%s)", args, err, string(out))
		}
	}
	return nil
}
