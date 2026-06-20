# claude-boot v2 — Go + Bubble Tea rewrite

**Date:** 2026-06-20
**Status:** Design approved, pending spec review

## Summary

`claude-boot` is an interactive launcher for Claude Code. Today it is a ~280-line
bash script (with three embedded `python3` heredocs) that shows an `fzf`
multi-select of installed plugins and skills, lets you disable any of them per
repo, persists those choices to the project `.claude/settings.json`, and then
launches `claude`.

This rewrite reimplements it in **Go** with a **Bubble Tea** TUI to add richer
interaction (grouped view, live filter, detail pane, profiles) and a new
**model + effort bootstrap**, while keeping fast startup and dropping the
external `fzf` dependency.

## Motivation

- The current script is "already a Python script in a bash costume" — all JSON
  work is done in embedded `python3`, and the bash↔python boundary is brittle
  (data passed via counted positional argv).
- New desired features (TUI grouping, search, profiles, preview pane, env
  bootstrap) push it well past where bash pays for itself.
- Go gives single-digit-ms startup, a single static binary, the mature Bubble
  Tea TUI ecosystem, and low maintenance friction — a better fit than Node
  (runtime cold-start) or Rust (dev friction with no perf payoff here).

## Goals

- Single-screen TUI with a quick-launch fast path.
- Categorized/grouped view (Plugins, Skills) with live type-to-filter.
- Detail pane showing the highlighted item's description.
- Named profiles (global) capturing a full launch config.
- Bootstrap `ANTHROPIC_MODEL` and `CLAUDE_CODE_EFFORT_LEVEL` for the launched
  `claude` process, prefilled from the current environment.
- Preserve existing persistence semantics for plugin/skill choices.
- Drop the `fzf` dependency.

## Non-goals

- Persisting model/effort to disk (child-env only).
- Persisting profiles per-repo (they are global).
- Modifying anything outside the project `.claude/settings.json` and the global
  profiles file.
- Wiring the built binary onto `PATH` (the user handles that).

## Stack & project location

- **Language/stack:** Go + Charm Bubble Tea / Bubbles / Lipgloss. Compiled to a
  single static binary. No `fzf` dependency.
- **Repo:** `~/code/me/claude-boot` — its own git repo (`git init`).
- **Build target:** `~/code/me/claude-boot/claude-boot` (gitignored binary),
  via a one-line `make` / `go build`.
- **PATH:** out of scope — the user wires it manually. `scripts/bin/claude-boot`
  (the bash version) is untouched and already backed up.
- **Tradeoff accepted:** edit→build→run replaces edit-in-place. Builds are
  sub-second.

### Repo layout

```
~/code/me/claude-boot/
├── .git/
├── CLAUDE.md            canonical doc: overview, build/run, conventions, architecture
├── README.md            → CLAUDE.md (symlink; GitHub renders it)
├── go.mod
├── Makefile             build → ./claude-boot
├── main.go              args (--restore, -p <name>), wiring
├── discover.go          read plugins, skills + descriptions, project settings, env
├── settings.go          read/modify/write project .claude/settings.json
├── profiles.go          load/save global profiles
├── tui.go               Bubble Tea model/update/view
├── launch.go            set env + exec claude
├── consts.go            model lineup + effort ladder
├── .gitignore           /claude-boot
└── *_test.go
```

## TUI design (single screen + quick-launch)

```
┌ claude-boot ─────────────────────────────────────────────┐
│ Model:  ▾ Opus 4.8         Effort: ▾ high                  │  ← Tab cycles regions
├───────────────────────────────────────────┬──────────────┤
│ Plugins (12)                                │ DETAIL        │
│  ✓ superpowers                              │ superpowers   │
│  ✓ slack                                    │ <description  │
│    frontend-design        ← unchecked = off │  from meta>   │
│ Skills (34)                                 │               │
│  ✓ artifactory                              │               │
│  ✓ caveman                                  │               │
│  …  (type to filter)                        │               │
├───────────────────────────────────────────┴──────────────┤
│ space toggle · / filter · p profiles · s save · ↵ launch · esc cancel │
└────────────────────────────────────────────────────────────┘
```

### Regions

- **Model / Effort fields** — dropdowns at the top.
  - Each offers an explicit top entry **"— inherit (don't set)"** so the user is
    never forced to override.
  - Model entries: inherit · Opus 4.8 (`claude-opus-4-8`) · Sonnet 4.6
    (`claude-sonnet-4-6`) · Haiku 4.5 (`claude-haiku-4-5-20251001`) · Fable 5
    (`claude-fable-5`).
  - Effort entries: inherit · low · medium · high · xhigh · max.
  - **Prefill:** if `ANTHROPIC_MODEL` / `CLAUDE_CODE_EFFORT_LEVEL` are already set
    in the environment, the matching entry is preselected; otherwise "inherit"
    is highlighted.
- **Grouped list** — a `Plugins (n)` section and a `Skills (n)` section. Each row
  is a checkbox (✓ = enabled/will run, unchecked = disabled for the session).
  Pre-disabled items (from existing project settings) start unchecked.
- **Live filter** — type-to-filter (`/` focuses the filter) across both sections
  simultaneously.
- **Detail pane** — shows the highlighted item's description. Source: skill →
  `SKILL.md` frontmatter `description`; plugin → its metadata description.

### Keybindings

- `Tab` — cycle regions (model field → effort field → list)
- `space` — toggle highlighted list item
- `/` — focus filter
- `p` — open profile picker (load)
- `s` — save current state as a named profile
- `Enter` — launch `claude` with current selections
- `Esc` — cancel without launching (exit 0)

## Profiles

- **Storage:** global, at `~/.claude/claude-boot/profiles.json`, so profiles
  apply in any repo.
- **Schema:** a profile is
  `{ name, model?, effort?, disabledPlugins[], disabledSkills[] }`.
  - `model` and `effort` are **optional**; when omitted they fall back to
    "inherit / pick per launch."
- **Save (`s`):** writes the current screen state (model, effort, disabled
  plugins/skills) as a named profile.
- **Load (`p`):** fills the model/effort fields and toggles from the profile;
  the user can still tweak before pressing Enter.
- **Quick-launch:** `claude-boot -p <name>` loads a profile and launches
  immediately, bypassing the TUI.

## Persistence semantics

| What             | Where it goes                                              | Persisted?                       |
|------------------|------------------------------------------------------------|----------------------------------|
| Disabled plugins | project `.claude/settings.json` → `enabledPlugins: false`  | yes (plain `claude` inherits)    |
| Disabled skills  | project `.claude/settings.json` → `skillOverrides: "off"`  | yes (plain `claude` inherits)    |
| Model / effort   | child env only (`ANTHROPIC_MODEL`, `CLAUDE_CODE_EFFORT_LEVEL`) | no                            |
| Profiles         | `~/.claude/claude-boot/profiles.json`                      | yes (global)                     |

- The settings writer preserves unrelated keys in `.claude/settings.json` and
  removes `enabledPlugins`/`skillOverrides` entries that are no longer disabled
  (matching today's behavior).
- `--restore` flag is preserved: strips `enabledPlugins` and `skillOverrides`
  from the project settings (deleting the file if it becomes empty).

## Discovery layer

Same sources as today, plus descriptions:

- **Plugins:** user-scoped entries from
  `~/.claude/plugins/installed_plugins.json` (only installs with
  `scope == "user"`), plus each plugin's metadata description for the detail
  pane.
- **Skills:** subdirectories of `~/.claude/skills/` (following symlinks,
  excluding dotfiles), plus each skill's `SKILL.md` frontmatter `description`.
- **Current state:** existing disabled plugins/skills read from the project
  `.claude/settings.json`; filtered to currently-installed items only.
- **Environment:** current `ANTHROPIC_MODEL` / `CLAUDE_CODE_EFFORT_LEVEL` for
  prefill.

## Launch flow

1. Parse args. `--restore` → run restore and exit. `-p <name>` → load profile,
   skip TUI, go to step 4.
2. Discover plugins/skills/settings/env. If nothing is configurable and no env
   override is desired, launch directly.
3. Run the TUI. `Esc` cancels (exit 0, no launch).
4. Write disabled plugins/skills to project `.claude/settings.json`.
5. Set `ANTHROPIC_MODEL` / `CLAUDE_CODE_EFFORT_LEVEL` in the child environment
   (only for entries that are not "inherit").
6. `exec claude`.

## Error handling

- Missing `installed_plugins.json` or `~/.claude/skills/` → treated as empty,
  not an error.
- Malformed JSON in project settings → start from empty settings rather than
  crash (matching today's `except` behavior).
- No plugins/skills found and no env change → launch `claude` directly.
- Missing/unreadable profiles file → treated as no profiles.
- `-p <name>` referencing an unknown profile → error message, non-zero exit, do
  not launch.

## Testing

Go unit tests for the non-TUI core:

- `discover` — parse fixture `installed_plugins.json`, fixture skills dirs with
  `SKILL.md` frontmatter, and current-settings extraction.
- `settings` — read → modify → write roundtrip; verify unrelated keys preserved,
  no-longer-disabled entries removed, empty-file deletion on restore.
- `profiles` — save/load roundtrip; optional model/effort handling.

TUI `Update` logic tested as a pure reducer where practical (toggle, filter,
region cycling, profile load).

## Open questions

None outstanding — design approved 2026-06-20.
