<img src="docs/icon.svg" width="64" height="64" align="left" alt="">

# tmux-alwayson

Keep Claude Code, Gemini CLI, OpenCode, Codex CLI, and other AI coding
agents running in tmux alive across reboots, with one command instead of a
dozen manual steps.

```bash
go install github.com/nahidspace/tmux-alwayson/cmd/tmux-alwayson@latest
tmux-alwayson install
tmux-alwayson status
```

![Claude Code session surviving a real reboot](docs/reboot-demo.gif)

`tmux-alwayson uninstall [--purge]` undoes it.

## Supported agents

```go
type Agent interface {
	Name() string
	Matches(cmdline string) bool
	ValidSession(sessionID, cwd string) bool
	NewestSession(cwd string) (id string, ok bool)
	ResumeCommand(sessionID string, extraArgs []string) []string
}
```

| Agent | Detection wired up | Live-verified |
|---|---|---|
| Claude Code | yes | yes, including a real unplanned reboot |
| OpenCode | yes | no |
| Codex CLI | yes | no |
| Pi | yes | no |
| Oh My Pi | yes | no |
| Grok | yes | no |
| Gemini CLI | not yet | no |
| Qwen Code | not yet | no |
| Hermes Agent | not yet | no |

"Wired up" means the bash hooks already detect that agent in a pane and
hand this tool a session ID to validate. The three "not yet" agents have a
working `Agent` implementation with no detection feeding it yet, add that
via the bash hooks or `Matches()`. Adding any new agent is implementing
this interface once and registering it in `cmd/tmux-alwayson/main.go`.

## Known limits

- Sudden power loss (not `sudo reboot`) can lose up to one save interval
  (5 min), since the guard only protects against a bad save, not against
  missing one entirely.
- Claude Code's one-time "trust this folder" prompt still needs a keypress.
