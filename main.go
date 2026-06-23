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
		fmt.Println("Restored: removed claude-boot keys from .claude/settings.json")
		return nil
	}

	configDir := ClaudeConfigDir(os.Getenv, home)
	globalModel, globalEffort := ReadGlobalDefaults(filepath.Join(configDir, "settings.json"))
	// Env vars take precedence over global settings.json for both the inherit
	// label and the suppress-on-write check.
	if v := os.Getenv(EnvModel); v != "" {
		globalModel = v
	}
	if v := os.Getenv(EnvEffort); v != "" {
		globalEffort = v
	}
	plugins, err := DiscoverPlugins(filepath.Join(configDir, "plugins", "installed_plugins.json"))
	if err != nil {
		return err
	}
	skills, err := DiscoverSkills(filepath.Join(configDir, "skills"))
	if err != nil {
		return err
	}
	disabled, err := ReadDisabled(sp)
	if err != nil {
		return err
	}
	profiles, err := LoadProfiles(DefaultProfilesPath(configDir))
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
		pm, pe := suppressGlobalDefaults(p.Model, p.Effort, globalModel, globalEffort)
		if err := WriteModelEffort(sp, pm, pe); err != nil {
			return err
		}
		return Launch(ChildEnv(env, p.Model, p.Effort))
	}

	// Interactive path. Persisted model/effort takes priority over env defaults.
	curModel, curEffort := EnvDefaults(os.Getenv)
	pm, pe, err := ReadModelEffort(sp)
	if err != nil {
		return err
	}
	if pm != "" {
		curModel = pm
	}
	if pe != "" {
		curEffort = pe
	}
	m := newModel(plugins, skills, disabled, curModel, curEffort, profiles, configDir, home, globalModel, globalEffort)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	fm := final.(model)
	dp, ds, modelID, effort, launch := fm.result()

	// Persist profiles regardless of launch/cancel so saves and deletes aren't lost on Escape.
	if err := SaveProfiles(DefaultProfilesPath(configDir), fm.profiles); err != nil {
		return err
	}

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
	wm, we := suppressGlobalDefaults(modelID, effort, globalModel, globalEffort)
	if err := WriteModelEffort(sp, wm, we); err != nil {
		return err
	}
	return Launch(ChildEnv(env, modelID, effort))
}

// suppressGlobalDefaults zeros out model/effort when they match the global
// ~/.claude/settings.json values so they aren't redundantly written to the
// project settings.json.
func suppressGlobalDefaults(model, effort, globalModel, globalEffort string) (string, string) {
	if model == globalModel {
		model = ""
	}
	if effort == globalEffort {
		effort = ""
	}
	return model, effort
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
