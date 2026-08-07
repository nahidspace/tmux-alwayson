// Package agent defines the interface tmux-agent-setup uses to detect,
// validate, and resume AI coding agent sessions inside tmux panes. Each
// concrete agent (Claude Code, OpenCode, Codex CLI, ...) implements Agent
// and registers itself; nothing outside this package needs to change when
// a new agent is added.
package agent

// Agent describes one AI coding CLI tmux-agent-setup knows how to detect,
// verify, and resume.
type Agent interface {
	// Name identifies the agent (e.g. "claude", "opencode"). Matches the
	// "tool" field already written by the existing save hooks, so this
	// stays compatible with sessions captured by the bash side of the
	// plugin.
	Name() string

	// Matches reports whether a process command line belongs to this
	// agent (e.g. "claude --resume abc", "/usr/bin/claude").
	Matches(cmdline string) bool

	// ValidSession reports whether sessionID is a real, resumable session
	// for this agent in the given working directory. "Real" means: exists,
	// and isn't a stub written before the agent's first real turn.
	ValidSession(sessionID, cwd string) bool

	// NewestSession returns the most recently active real session for cwd,
	// if any. Used as a fallback when a captured session ID fails
	// ValidSession -- e.g. the classic case of a SessionStart hook firing
	// before the transcript exists, and the process dying before it does.
	NewestSession(cwd string) (id string, ok bool)

	// ResumeCommand builds the argv (not a shell string -- callers quote
	// as needed for their target shell) to resume sessionID, appending any
	// extra CLI args the original invocation had.
	ResumeCommand(sessionID string, extraArgs []string) []string
}

// Registry looks up an Agent by name. Adding a new agent is: implement
// Agent, then Register it once during program startup.
type Registry struct {
	agents map[string]Agent
}

func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]Agent)}
}

func (r *Registry) Register(a Agent) {
	r.agents[a.Name()] = a
}

func (r *Registry) Get(name string) (Agent, bool) {
	a, ok := r.agents[name]
	return a, ok
}

// Detect returns the first registered Agent whose Matches reports true for
// cmdline, if any. Registration order breaks ties, so register more
// specific agents before more general ones if that ever matters.
func (r *Registry) Detect(cmdline string) (Agent, bool) {
	for _, a := range r.agents {
		if a.Matches(cmdline) {
			return a, true
		}
	}
	return nil, false
}
