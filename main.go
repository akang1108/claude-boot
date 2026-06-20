package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

type cliArgs struct {
	restore bool
	profile string
}

func parseArgs(argv []string) (cliArgs, error) {
	var a cliArgs
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--restore":
			a.restore = true
		case "-p", "--profile":
			if i+1 >= len(argv) {
				return a, fmt.Errorf("%s requires a profile name", argv[i])
			}
			i++
			a.profile = argv[i]
		default:
			return a, fmt.Errorf("unknown argument: %s", argv[i])
		}
	}
	return a, nil
}

func settingsPath(cwd string) string {
	return filepath.Join(cwd, ".claude", "settings.json")
}

// run orchestrates everything except the interactive TUI render. The TUI path
// (no flags) builds the model and runs the Bubble Tea program; non-interactive
// paths (--restore, -p) are fully covered here and are unit-tested.
func run(args cliArgs, env []string, home, cwd string) error {
	sp := settingsPath(cwd)

	if args.restore {
		if err := Restore(sp); err != nil {
			return err
		}
		fmt.Println("Restored: removed enabledPlugins and skillOverrides from .claude/settings.json")
		return nil
	}

	plugins, err := DiscoverPlugins(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"))
	if err != nil {
		return err
	}
	skills, err := DiscoverSkills(filepath.Join(home, ".claude", "skills"))
	if err != nil {
		return err
	}
	disabled, err := ReadDisabled(sp)
	if err != nil {
		return err
	}
	profiles, err := LoadProfiles(DefaultProfilesPath(home))
	if err != nil {
		return err
	}

	// Quick-launch: load a profile and skip the TUI.
	if args.profile != "" {
		p, ok := FindProfile(profiles, args.profile)
		if !ok {
			return fmt.Errorf("profile not found: %s", args.profile)
		}
		if err := os.MkdirAll(filepath.Dir(sp), 0o755); err != nil {
			return err
		}
		if err := WriteDisabled(sp, names(plugins), p.DisabledPlugins, names(skills), p.DisabledSkills); err != nil {
			return err
		}
		return Launch(ChildEnv(env, p.Model, p.Effort))
	}

	// Interactive path.
	curModel, curEffort := EnvDefaults(os.Getenv)
	m := newModel(plugins, skills, disabled, curModel, curEffort, profiles)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	fm := final.(model)
	dp, ds, modelID, effort, launch := fm.result()
	if !launch {
		fmt.Println("Cancelled.")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(sp), 0o755); err != nil {
		return err
	}
	if err := WriteDisabled(sp, names(plugins), dp, names(skills), ds); err != nil {
		return err
	}
	// Persist any profiles saved during the session.
	for _, p := range fm.profiles {
		if err := SaveProfile(DefaultProfilesPath(home), p); err != nil {
			return err
		}
	}
	return Launch(ChildEnv(env, modelID, effort))
}

func names(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func main() {
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-boot:", err)
		os.Exit(2)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-boot:", err)
		os.Exit(1)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-boot:", err)
		os.Exit(1)
	}
	if err := run(args, os.Environ(), home, cwd); err != nil {
		fmt.Fprintln(os.Stderr, "claude-boot:", err)
		os.Exit(1)
	}
}
