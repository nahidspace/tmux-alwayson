# tmux-alwayson — keep Claude Code, OpenCode, and Codex CLI sessions alive through a reboot

If you've ever rebooted a box running **Claude Code**, **OpenCode**, or
**Codex CLI** inside `tmux` and come back to a blank prompt instead of your
conversation, this is for it. `tmux-alwayson` is a one-command installer
that makes an AI coding agent's tmux session **survive both a plain tmux
restart and a full system reboot** — including on a headless box like a
Raspberry Pi with nobody logged in.

```bash
go install github.com/nahidspace/tmux-alwayson/cmd/tmux-alwayson@latest
tmux-alwayson install
tmux-alwayson status
```

## Why does my Claude Code / tmux session disappear after a reboot?

Three separate things have to go right for an AI agent's tmux session to
come back on its own, and by default none of them do:

1. **tmux itself has to start on boot.** Nothing does this without a
   systemd unit — `tmux a` failing with "no server running" after a reboot
   almost always means this step is missing.
2. **The session layout has to be saved reliably**, including *while the
   system is shutting down* — not just periodically while it happens to be
   convenient.
3. **The agent's own session ID has to still be valid.** Claude Code's
   `SessionStart` hook records a session ID the instant the process
   launches, before its transcript file exists — kill it before the first
   reply and you get an ID that looks fine but makes `--resume` fail with
   `No conversation found with session ID: ...`.

`tmux-alwayson` handles all three, and does so having actually rebooted a
real machine to prove each one — not by reading the underlying plugins'
source and assuming.

## What it's built on

[`tmux-resurrect`](https://github.com/tmux-plugins/tmux-resurrect) and
[`tmux-continuum`](https://github.com/tmux-plugins/tmux-continuum) already
save and restore tmux layouts; the agent-detection hooks in
[`tmux-assistant-resurrect`](https://github.com/nahidspace/tmux-assistant-resurrect)
already know how to spot Claude Code, OpenCode, and Codex CLI running in a
pane and capture their session IDs. `tmux-alwayson` installs and
orchestrates all of that as one command, and adds the reliability layer
none of them provide on their own: a boot-safe systemd setup, a save
mechanism that doesn't depend on tmux's own backgrounded job system, and a
guard that rejects a bad save or a stale session ID before a restore ever
sees it.

## `tmux-alwayson install` does the whole setup

1. Clones the agent-hooks repo and [TPM](https://github.com/tmux-plugins/tpm)
   if missing.
2. Writes the `~/.tmux.conf` block — plugins, save/restore hooks,
   `@continuum-boot`, and `@continuum-save-interval '0'` (this tool owns
   periodic saving now, see below for why).
3. Runs TPM's plugin install and the hooks repo's own agent-hook installer.
4. Copies itself to `~/.local/bin/tmux-alwayson` so systemd has a stable
   path to call.
5. Writes and enables three `systemd --user` units: `tmux.service` (starts
   tmux on boot, guard-saves on shutdown) and
   `tmux-alwayson-save.timer`/`.service` (periodic saves every 5 minutes).
6. `loginctl enable-linger`, so those units actually run with nobody logged
   in — the exact situation after most unattended reboots.

Everything is idempotent; run `install` again any time. `tmux-alwayson
uninstall` (optionally with `--purge`) undoes it.

## The three bugs that make the plugins' own defaults not enough

- **`tmux-resurrect`'s restore cleanup unconditionally kills a session
  literally named `0`** once it's done restoring your real sessions.
  Sensible when `0` is a disposable leftover placeholder; fatal when `0` is
  the *only* session that exists — exactly what a bare
  `tmux new-session -d` on boot produces, killing the whole server seconds
  after startup. Fixed by giving the boot placeholder any other name.
- **`systemd`'s `Type=forking` PID-guessing is flaky under real boot-time
  CPU contention** and can tear a perfectly healthy unit down thinking it
  already exited. Fixed with `Type=oneshot` + `RemainAfterExit=yes`, which
  never depends on guessing a long-lived PID.
- **`tmux-continuum`'s periodic autosave runs as a job backgrounded off a
  status-bar `#(...)` interpolation** — fine for a quick status query, not
  guaranteed to run a multi-step save script to completion. Fixed with a
  systemd timer that calls the save directly and synchronously instead.

## Adding OpenCode, Codex CLI, or any other agent

Whether a captured session ID is still real, and what to fall back to if
it isn't, is defined once behind an interface — not hard-coded per agent:

```go
type Agent interface {
	Name() string
	Matches(cmdline string) bool
	ValidSession(sessionID, cwd string) bool
	NewestSession(cwd string) (id string, ok bool)
	ResumeCommand(sessionID string, extraArgs []string) []string
}
```

`Claude`, `OpenCode`, and `Codex` (`internal/agent/`) are implemented and
tested today. Adding another agent — or wiring up `Hermes`, which is
implemented but not yet connected to detection — is implementing this
interface once and registering it in `cmd/tmux-alwayson/main.go`. The
installer and the save guard don't change.

## FAQ

**Does this work on a Raspberry Pi?** Yes — built and tested on one running
Kali, headless, powered off a battery pack that occasionally cuts out
without warning. That's the actual motivating case.

**Does it survive a hard power loss, not just `sudo reboot`?** Mostly.
Sudden power loss skips the graceful shutdown save, so you can lose
whatever happened since the last periodic save (five minutes by default) —
but the guard means that periodic save is never corrupted or silently
empty, only ever as stale as the interval.

**Does it support OpenCode or Codex CLI?** Their `Agent` implementations
exist and are tested; full detection still depends on the bash hooks in
`tmux-assistant-resurrect` recognizing them in a pane, which it already
does for both.

**Why not just use `tmux-continuum`'s autosave?** It's a backgrounded job
tied to tmux's own status-bar rendering, not a reliably-run save script —
see "The three bugs" above.
