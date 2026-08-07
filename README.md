# tmux-alwayson — keep Claude Code, Gemini CLI, OpenCode, Codex CLI, and more alive through a reboot

If you've ever rebooted a box running **Claude Code**, **Gemini CLI**,
**Qwen Code**, **OpenCode**, or **Codex CLI** inside `tmux` and come back to
a blank prompt instead of your conversation, this is for it.
`tmux-alwayson` is a one-command installer that makes an AI coding agent's
tmux session **survive both a plain tmux restart and a full system
reboot** — including on a headless box like a Raspberry Pi with nobody
logged in.

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

## Supported agents

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

Nine agents are implemented and tested in `internal/agent/`:

| Agent | Session storage | Live-verified | Detection wired up |
|---|---|---|---|
| Claude Code | per-session `.jsonl` transcript | yes, repeatedly, including a real unplanned reboot | yes |
| OpenCode | SQLite (`opencode.db`) | no | yes |
| Codex CLI | SQLite `threads` table, or rollout `.jsonl` (older versions) | no | yes |
| Pi | per-session `.jsonl` under a cwd-scoped dir | no | yes |
| Oh My Pi | per-session `.jsonl`, default profile only | no | yes |
| Grok (xAI) | JSON registry (`active_sessions.json`) | no | yes |
| Gemini CLI | per-session `.jsonl`, scanned by content | no | not yet |
| Qwen Code | per-session `.jsonl`, scanned by content | no | not yet |
| Hermes Agent | SQLite (`state.db`) | no | not yet |

"Detection wired up" means the bash hooks in `tmux-assistant-resurrect`
already recognize that agent running in a pane and populate a sidecar
entry for it — at that point this tool's `Agent` implementation validates
and repairs what got captured. For the three marked "not yet," the
`ValidSession`/`NewestSession` logic is ready, but nothing produces a
sidecar entry for them yet; that needs either an addition to those bash
hooks, or `tmux-alwayson` doing its own process scan via the already-built
`Matches()` method. Only Claude Code has been exercised against a real,
running install through this tool end to end — the rest are implemented
from each project's documented (or, for Codex/Gemini/Qwen, source-code-read)
session storage format.

Adding another agent, or finishing the wiring for one of the three above,
is implementing this interface once and registering it in
`cmd/tmux-alwayson/main.go`. The installer and the save guard don't change.

## FAQ

**Does this work on a Raspberry Pi?** Yes — built and tested on one running
Kali, headless, powered off a battery pack that occasionally cuts out
without warning. That's the actual motivating case.

**Does it survive a hard power loss, not just `sudo reboot`?** Mostly.
Sudden power loss skips the graceful shutdown save, so you can lose
whatever happened since the last periodic save (five minutes by default) —
but the guard means that periodic save is never corrupted or silently
empty, only ever as stale as the interval.

**Does it support OpenCode, Codex CLI, Gemini CLI, or Qwen Code?**
OpenCode, Codex CLI, Pi, Oh My Pi, and Grok are detected end to end today.
Gemini CLI and Qwen Code have working `Agent` implementations but aren't
wired into detection yet — see the agent table above.

**Why not just use `tmux-continuum`'s autosave?** It's a backgrounded job
tied to tmux's own status-bar rendering, not a reliably-run save script —
see "The three bugs" above.
