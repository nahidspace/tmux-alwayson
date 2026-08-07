// Command tmux-alwayson installs and runs the pieces needed for an AI
// coding agent's tmux session to survive both a plain tmux restart and a
// full system reboot: TPM + tmux-resurrect + tmux-continuum, the agent
// hooks that detect and capture sessions, a boot-safe systemd unit, a
// reliable periodic-save timer, and a Go-native guard that protects the
// last good save and repairs stale session IDs before they ever reach a
// restore.
//
// Detection of which agent is running in a pane, and how to extract its
// session ID, is deliberately behind the agent.Agent interface (see
// internal/agent) -- Claude Code is the only implementation today, but
// adding OpenCode, Codex CLI, or anything else is adding one more
// implementation, not touching the installer or the guard.
package main

import (
	"fmt"
	"os"

	"github.com/nahidspace/tmux-alwayson/internal/agent"
	guardpkg "github.com/nahidspace/tmux-alwayson/internal/guard"
	"github.com/nahidspace/tmux-alwayson/internal/setup"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "install":
		err = runInstall()
	case "status":
		err = runStatus()
	case "guarded-save":
		err = runGuardedSave()
	case "uninstall":
		err = runUninstall(hasFlag("--purge"))
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "tmux-alwayson:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`tmux-alwayson - make an AI coding agent's tmux session survive reboots

Usage:
  tmux-alwayson install       one-shot setup: TPM, plugins, agent hooks,
                               tmux.conf, systemd units, linger
  tmux-alwayson status        check what's installed and currently active
  tmux-alwayson guarded-save  run one save cycle (used by the systemd timer
                               and by tmux.service's own shutdown save --
                               not normally run by hand)
  tmux-alwayson uninstall     disable and remove the systemd units and the
                               tmux.conf block this tool added
  tmux-alwayson uninstall --purge
                               also remove the TPM/plugin/hooks-repo clones
                               and disable linger (leaves any agent-side
                               hook registration, e.g. ~/.claude/settings.json,
                               untouched -- see README)`)
}

func hasFlag(name string) bool {
	for _, a := range os.Args[2:] {
		if a == name {
			return true
		}
	}
	return false
}

func registry() *agent.Registry {
	reg := agent.NewRegistry()
	reg.Register(agent.Claude{})
	reg.Register(agent.OpenCode{})
	reg.Register(agent.Codex{})
	reg.Register(agent.Hermes{})
	reg.Register(agent.Pi{})
	reg.Register(agent.OMP{})
	reg.Register(agent.Grok{})
	reg.Register(agent.Gemini{})
	reg.Register(agent.Qwen{})
	return reg
}

func runInstall() error {
	if err := setup.RequireBinaries("tmux", "git", "bash", "systemctl", "loginctl"); err != nil {
		return err
	}

	p, err := setup.DefaultPaths()
	if err != nil {
		return err
	}

	step := func(label string, fn func() error) error {
		fmt.Print("-> " + label + " ... ")
		if err := fn(); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("%s: %w", label, err)
		}
		fmt.Println("ok")
		return nil
	}

	if err := step("cloning agent hooks", func() error {
		_, err := setup.EnsureCloned(setup.DefaultHooksRepoURL, p.HooksRepoDir)
		return err
	}); err != nil {
		return err
	}
	if err := step("installing TPM", func() error {
		_, err := setup.EnsureCloned(setup.TPMURL, p.TPMDir)
		return err
	}); err != nil {
		return err
	}
	if err := step("writing tmux.conf", func() error { return setup.PatchTmuxConf(p) }); err != nil {
		return err
	}
	if err := step("installing TPM plugins (tmux-resurrect, tmux-continuum)", func() error {
		return setup.InstallTPMPlugins(p)
	}); err != nil {
		return err
	}
	if err := step("installing agent hooks (Claude, OpenCode)", func() error {
		return setup.InstallHooks(p)
	}); err != nil {
		return err
	}

	var selfPath string
	if err := step("installing tmux-alwayson to ~/.local/bin", func() error {
		selfPath, err = setup.InstallSelf(p)
		return err
	}); err != nil {
		return err
	}
	if err := step("writing systemd units", func() error {
		return setup.WriteSystemdUnits(p, selfPath)
	}); err != nil {
		return err
	}
	if err := step("enabling linger (survive reboots with no login)", setup.EnableLinger); err != nil {
		return err
	}
	if err := step("enabling systemd units", setup.EnableSystemdUnits); err != nil {
		return err
	}

	fmt.Println("\nInstall complete. Reload tmux config in any live session:")
	fmt.Println("  tmux source-file ~/.tmux.conf")
	fmt.Println("\nCheck everything is wired up:")
	fmt.Println("  tmux-alwayson status")
	return nil
}

func runStatus() error {
	p, err := setup.DefaultPaths()
	if err != nil {
		return err
	}
	fmt.Print(setup.Status(p))

	gp, err := guardpkg.DefaultPaths()
	if err == nil {
		fmt.Println()
		fmt.Println(setup.LastSaveSummary(gp.ResurrectDataDir))
	}
	return nil
}

func runGuardedSave() error {
	paths, err := guardpkg.DefaultPaths()
	if err != nil {
		return err
	}
	summary, err := guardpkg.Run(paths, registry())
	if err != nil {
		return err
	}
	fmt.Println(summary)
	return nil
}

func runUninstall(purge bool) error {
	p, err := setup.DefaultPaths()
	if err != nil {
		return err
	}

	step := func(label string, fn func() error) error {
		fmt.Print("-> " + label + " ... ")
		if err := fn(); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("%s: %w", label, err)
		}
		fmt.Println("ok")
		return nil
	}

	if err := step("disabling and removing systemd units", setup.DisableSystemdUnits); err != nil {
		return err
	}
	if err := step("removing tmux.conf block", func() error { return setup.StripTmuxConf(p) }); err != nil {
		return err
	}

	if !purge {
		fmt.Println("\nDone. TPM/plugin/hooks-repo clones and linger were left in place -- rerun with --purge to remove those too.")
		return nil
	}

	if err := step("removing TPM, plugin, and hooks-repo clones", func() error { return setup.PurgeClones(p) }); err != nil {
		return err
	}
	fmt.Println("\nDone. Agent-side hook registration (e.g. ~/.claude/settings.json) was left as-is -- see README.")
	return nil
}
