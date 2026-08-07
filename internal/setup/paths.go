package setup

import (
	"os"
	"path/filepath"
)

const (
	DefaultHooksRepoURL = "https://github.com/nahidspace/tmux-assistant-resurrect.git"
	TPMURL              = "https://github.com/tmux-plugins/tpm"
)

// Paths collects every filesystem location tmux-alwayson reads or writes.
// Detection/capture logic (which tools run in which panes, how to extract
// their session IDs) still lives in the tmux-assistant-resurrect hooks --
// this tool installs and orchestrates them, and adds the Go-native guard
// and systemd wiring on top.
type Paths struct {
	Home           string
	TmuxConf       string
	TPMDir         string
	ResurrectDir   string // ~/.tmux/plugins/tmux-resurrect
	ContinuumDir   string // ~/.tmux/plugins/tmux-continuum
	HooksRepoDir   string // ~/tmux-assistant-resurrect (claude/opencode hooks + save/restore scripts)
	SelfBinDir     string // ~/.local/bin
	SystemdUserDir string // ~/.config/systemd/user
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Home:           home,
		TmuxConf:       filepath.Join(home, ".tmux.conf"),
		TPMDir:         filepath.Join(home, ".tmux", "plugins", "tpm"),
		ResurrectDir:   filepath.Join(home, ".tmux", "plugins", "tmux-resurrect"),
		ContinuumDir:   filepath.Join(home, ".tmux", "plugins", "tmux-continuum"),
		HooksRepoDir:   filepath.Join(home, "tmux-assistant-resurrect"),
		SelfBinDir:     filepath.Join(home, ".local", "bin"),
		SystemdUserDir: filepath.Join(home, ".config", "systemd", "user"),
	}, nil
}
