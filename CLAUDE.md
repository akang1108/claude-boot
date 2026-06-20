# claude-boot

Interactive launcher for Claude Code. Shows a TUI to toggle installed plugins and
skills, bootstrap the model + thinking-effort environment variables, and apply
named profiles before launching `claude`.

## Build & run

```bash
make build        # builds ./claude-boot
./claude-boot      # interactive launch
./claude-boot -p <profile>   # load a profile and launch immediately
./claude-boot --restore      # strip claude-boot keys from project .claude/settings.json
```

## Architecture

Single `package main`. Pure logic in focused, unit-tested files:

- consts.go — model lineup, effort ladder, env var names
- discover.go — read user plugins, local skills (+descriptions), current env
- settings.go — read/modify/write project .claude/settings.json
- profiles.go — load/save ~/.claude/claude-boot/profiles.json
- launch.go — build child env, exec claude
- tui.go — Bubble Tea model (Update reducer + View)
- main.go — arg parsing + orchestration

## Conventions

- Disabled plugins → enabledPlugins[name]=false; disabled skills → skillOverrides[name]="off"
  in the PROJECT .claude/settings.json (persisted; plain `claude` inherits).
- Model/effort (ANTHROPIC_MODEL, CLAUDE_CODE_EFFORT_LEVEL) are set on the child
  process only — never written to disk.
- Profiles are global.
- Tests: stdlib `testing`; TUI tested by feeding tea.KeyMsg to Update.

## Releasing

See [docs/releasing.md](docs/releasing.md). Every push to `main` auto-increments
the patch version and publishes a GitHub release with three pre-built binaries.

## Notes

- **Always shows the TUI.** Unlike the original bash script (which `exec`'d
  claude directly when there were no plugins/skills to toggle), v2 always shows
  the single-screen TUI so the model/effort pickers stay available. Intentional.
- **Go toolchain floor.** The Charm TUI deps (bubbletea / bubbles / lipgloss)
  require a recent Go, so go.mod's floor is whatever they pull in (currently
  go 1.24.x) — higher than the 1.21 originally targeted. Fine for a single-user
  dev tool.
