# tmux-alwayson

Make an AI coding agent's tmux session (Claude Code today; more pluggable
via the `Agent` interface) survive a tmux restart *and* a full reboot —
one command instead of a dozen manual steps.

Built on [`tmux-resurrect`](https://github.com/tmux-plugins/tmux-resurrect),
[`tmux-continuum`](https://github.com/tmux-plugins/tmux-continuum), and the
agent-detection hooks in
[`tmux-assistant-resurrect`](https://github.com/nahidspace/tmux-assistant-resurrect).
This tool installs and orchestrates those, and adds what they don't provide
on their own: a boot-safe systemd setup, a save mechanism that doesn't
depend on tmux's own backgrounded job system, and a guard that catches bad
saves and stale session IDs before a restore ever sees them.

## Install

```bash
go install github.com/nahidspace/tmux-alwayson/cmd/tmux-alwayson@latest
tmux-alwayson install
tmux-alwayson status
```

Requires `tmux`, `git`, `bash`, `systemctl`, `loginctl` on PATH — `install`
checks and refuses to guess around anything missing. Idempotent: safe to
run again.

## What `install` does

1. Clones the agent-hooks repo and TPM if missing.
2. Writes the `~/.tmux.conf` block (plugins, hooks, `@continuum-boot`,
   `@continuum-save-interval '0'` — this tool owns periodic saving now).
3. Runs TPM's plugin install and the hooks repo's own installer.
4. Copies itself to `~/.local/bin/tmux-alwayson`.
5. Writes and enables three systemd `--user` units: `tmux.service` (boots
   tmux with a safely-named placeholder, guarded-saves on shutdown),
   `tmux-alwayson-save.timer`/`.service` (real periodic saves).
6. `loginctl enable-linger`.

## Why not just the plugins' own defaults

- `tmux-resurrect`'s restore cleanup kills a session literally named `0` —
  fine when it's a leftover placeholder, fatal when it's the *only*
  session, which is exactly what a bare `tmux new-session -d` on boot
  produces. Fixed by naming the placeholder.
- `systemd`'s `Type=forking` PID-guessing is flaky under real boot-time
  load. Fixed with `Type=oneshot` + `RemainAfterExit=yes`.
- `tmux-continuum`'s periodic autosave runs as a backgrounded job off a
  status-bar interpolation — not guaranteed to finish. Fixed with a
  systemd timer that calls the save directly.

All three were caught by actually rebooting a real box, not by reading the
source and assuming.

## The `Agent` interface

`guarded-save` validates every captured session ID before trusting it —
Claude Code's `SessionStart` hook fires before its transcript exists, so an
ID can look valid and back a conversation that never happened. That check,
and the newest-real-session fallback, are defined once:

```go
type Agent interface {
	Name() string
	Matches(cmdline string) bool
	ValidSession(sessionID, cwd string) bool
	NewestSession(cwd string) (id string, ok bool)
	ResumeCommand(sessionID string, extraArgs []string) []string
}
```

`Claude` (`internal/agent/claude.go`) is the only implementation so far.
Adding OpenCode, Codex CLI, etc. is implementing this once and registering
it in `cmd/tmux-alwayson/main.go` — the installer and guard don't change.

## Not handled (yet)

- Agent *detection* itself still comes from the bash hooks in
  `tmux-assistant-resurrect` — this tool validates what they capture, it
  doesn't replace their detection.
- Sudden power loss still loses at most one save interval — inherent to
  any interval-based save.
- Claude Code's one-time "trust this folder" prompt still needs a keypress.
