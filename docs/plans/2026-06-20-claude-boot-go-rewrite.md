# claude-boot Go + Bubble Tea Rewrite — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reimplement `claude-boot` as a fast Go binary with a Bubble Tea TUI that lets you toggle plugins/skills, bootstrap model + effort env vars, and apply named profiles before launching `claude`.

**Architecture:** A single `package main` Go program. Pure logic lives in small focused files (`consts`, `settings`, `discover`, `profiles`, `launch`) each unit-tested with stdlib `testing`. The TUI (`tui.go`) is a Bubble Tea model whose `Update` is a pure reducer tested directly by feeding `tea.KeyMsg` values; `main.go` parses args and wires everything together, ending in `exec claude`.

**Tech Stack:** Go (1.21+), Charm Bubble Tea + Bubbles (textinput) + Lipgloss. Standard library `encoding/json`, `os`, `syscall`.

## Global Constraints

- Module path: `claude-boot` (local module name; can be repointed to the GitHub URL later — does not affect intra-package code since everything is `package main`).
- Go version floor: **1.21**.
- Dependencies limited to: `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/lipgloss`. No YAML library — frontmatter parsed by hand. No other third-party deps.
- TUI uses the **classic Bubble Tea Model interface**: `Init() tea.Cmd`, `Update(tea.Msg) (tea.Model, tea.Cmd)`, `View() string`.
- Env var names, copied verbatim: `ANTHROPIC_MODEL`, `CLAUDE_CODE_EFFORT_LEVEL`.
- Model lineup (label → id): Opus 4.8 → `claude-opus-4-8`; Sonnet 4.6 → `claude-sonnet-4-6`; Haiku 4.5 → `claude-haiku-4-5-20251001`; Fable 5 → `claude-fable-5`. Each picker has an "inherit (don't set)" sentinel as index 0 (empty value).
- Effort ladder: `low`, `medium`, `high`, `xhigh`, `max` (plus the inherit sentinel at index 0).
- Persistence: disabled plugins → `enabledPlugins[name]=false`; disabled skills → `skillOverrides[name]="off"` in **project** `.claude/settings.json`. Model/effort are **child-env only, never written to disk**. Profiles stored at `~/.claude/claude-boot/profiles.json`.
- The built binary `./claude-boot` is gitignored. Nothing outside the project `.claude/settings.json` and the global profiles file is ever modified.
- Commit messages: one-liners, no tool attribution.

## File Structure

```
~/code/me/claude-boot/
├── go.mod
├── .gitignore           /claude-boot
├── Makefile             build → ./claude-boot
├── CLAUDE.md            canonical doc
├── README.md            → CLAUDE.md (symlink)
├── consts.go            model lineup + effort ladder + env var names  (Task 1)
├── consts_test.go                                                     (Task 1)
├── settings.go          read/modify/write project .claude/settings.json (Task 2)
├── settings_test.go                                                   (Task 2)
├── discover.go          read plugins, skills+descriptions, env        (Task 3)
├── discover_test.go                                                   (Task 3)
├── profiles.go          load/save global profiles                     (Task 4)
├── profiles_test.go                                                   (Task 4)
├── launch.go            build child env + exec claude                 (Task 5)
├── launch_test.go                                                     (Task 5)
├── tui.go               Bubble Tea model: Update reducer + View       (Tasks 6, 7)
├── tui_test.go                                                        (Tasks 6, 7)
└── main.go              arg parsing + orchestration                   (Task 8)
    main_test.go                                                       (Task 8)
```

Task ordering keeps each task compiling: Tasks 1–5 are stdlib-only; Bubble Tea deps are added in Task 6.

---

### Task 1: Scaffold + constants

**Files:**
- Create: `go.mod`, `.gitignore`, `Makefile`, `CLAUDE.md`, `README.md` (symlink), `consts.go`
- Test: `consts_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type ModelChoice struct { Label string; ID string }`
  - `var Models []ModelChoice` (index 0 = inherit sentinel, `ID == ""`)
  - `var Efforts []string` (index 0 = `""` inherit sentinel)
  - `const EnvModel = "ANTHROPIC_MODEL"`, `const EnvEffort = "CLAUDE_CODE_EFFORT_LEVEL"`
  - `func ModelIndexByID(id string) int` — returns matching `Models` index, else 0
  - `func EffortIndex(v string) int` — returns matching `Efforts` index, else 0

- [ ] **Step 1: Initialize module and project files**

```bash
cd ~/code/me/claude-boot
go mod init claude-boot
printf '/claude-boot\n' > .gitignore
```

Create `Makefile`:

```makefile
BINARY := claude-boot

.PHONY: build test fmt clean
build:
	go build -o $(BINARY) .

test:
	go test ./...

fmt:
	go fmt ./...

clean:
	rm -f $(BINARY)
```

Create `CLAUDE.md`:

```markdown
# claude-boot

Interactive launcher for Claude Code. Shows a TUI to toggle installed plugins and
skills, bootstrap the model + thinking-effort environment variables, and apply
named profiles before launching `claude`.

## Build & run

    make build        # builds ./claude-boot
    ./claude-boot      # interactive launch
    ./claude-boot -p <profile>   # load a profile and launch immediately
    ./claude-boot --restore      # strip claude-boot keys from project .claude/settings.json

## Architecture

Single `package main`. Pure logic in focused, unit-tested files:
- consts.go   — model lineup, effort ladder, env var names
- discover.go — read user plugins, local skills (+descriptions), current env
- settings.go — read/modify/write project .claude/settings.json
- profiles.go — load/save ~/.claude/claude-boot/profiles.json
- launch.go   — build child env, exec claude
- tui.go      — Bubble Tea model (Update reducer + View)
- main.go     — arg parsing + orchestration

## Conventions

- Disabled plugins → enabledPlugins[name]=false; disabled skills → skillOverrides[name]="off"
  in the PROJECT .claude/settings.json (persisted; plain `claude` inherits).
- Model/effort (ANTHROPIC_MODEL, CLAUDE_CODE_EFFORT_LEVEL) are set on the child
  process only — never written to disk.
- Profiles are global.
- Tests: stdlib `testing`; TUI tested by feeding tea.KeyMsg to Update.
```

Create the `README.md` symlink:

```bash
ln -s CLAUDE.md README.md
```

- [ ] **Step 2: Write the failing test**

`consts_test.go`:

```go
package main

import "testing"

func TestModelsLineup(t *testing.T) {
	if Models[0].ID != "" {
		t.Fatalf("index 0 must be inherit sentinel, got %q", Models[0].ID)
	}
	want := map[string]string{
		"Opus 4.8":   "claude-opus-4-8",
		"Sonnet 4.6": "claude-sonnet-4-6",
		"Haiku 4.5":  "claude-haiku-4-5-20251001",
		"Fable 5":    "claude-fable-5",
	}
	got := map[string]string{}
	for _, m := range Models[1:] {
		got[m.Label] = m.ID
	}
	for label, id := range want {
		if got[label] != id {
			t.Errorf("model %q: want id %q, got %q", label, id, got[label])
		}
	}
}

func TestEffortsLadder(t *testing.T) {
	if Efforts[0] != "" {
		t.Fatalf("index 0 must be inherit sentinel, got %q", Efforts[0])
	}
	want := []string{"low", "medium", "high", "xhigh", "max"}
	for i, v := range want {
		if Efforts[i+1] != v {
			t.Errorf("effort %d: want %q, got %q", i+1, v, Efforts[i+1])
		}
	}
}

func TestModelIndexByID(t *testing.T) {
	if got := ModelIndexByID("claude-sonnet-4-6"); Models[got].ID != "claude-sonnet-4-6" {
		t.Errorf("ModelIndexByID returned %d", got)
	}
	if got := ModelIndexByID("nonexistent"); got != 0 {
		t.Errorf("unknown id should map to 0, got %d", got)
	}
}

func TestEffortIndex(t *testing.T) {
	if got := EffortIndex("high"); Efforts[got] != "high" {
		t.Errorf("EffortIndex returned %d", got)
	}
	if got := EffortIndex(""); got != 0 {
		t.Errorf("empty should map to 0, got %d", got)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./... -run 'TestModels|TestEfforts|TestModelIndex|TestEffortIndex' -v`
Expected: FAIL — `undefined: Models` (and friends).

- [ ] **Step 4: Write minimal implementation**

`consts.go`:

```go
package main

const (
	EnvModel  = "ANTHROPIC_MODEL"
	EnvEffort = "CLAUDE_CODE_EFFORT_LEVEL"
)

// ModelChoice is one entry in the model picker.
type ModelChoice struct {
	Label string // friendly label shown in the TUI
	ID    string // ANTHROPIC_MODEL value; "" means "inherit / don't set"
}

// Models is the picker lineup. Index 0 is the inherit sentinel.
var Models = []ModelChoice{
	{Label: "— inherit (don't set)", ID: ""},
	{Label: "Opus 4.8", ID: "claude-opus-4-8"},
	{Label: "Sonnet 4.6", ID: "claude-sonnet-4-6"},
	{Label: "Haiku 4.5", ID: "claude-haiku-4-5-20251001"},
	{Label: "Fable 5", ID: "claude-fable-5"},
}

// Efforts is the CLAUDE_CODE_EFFORT_LEVEL ladder. Index 0 is the inherit sentinel.
var Efforts = []string{"", "low", "medium", "high", "xhigh", "max"}

// ModelIndexByID returns the Models index whose ID matches id, or 0 (inherit).
func ModelIndexByID(id string) int {
	for i, m := range Models {
		if m.ID == id && id != "" {
			return i
		}
	}
	return 0
}

// EffortIndex returns the Efforts index matching v, or 0 (inherit).
func EffortIndex(v string) int {
	for i, e := range Efforts {
		if e == v && v != "" {
			return i
		}
	}
	return 0
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod .gitignore Makefile CLAUDE.md README.md consts.go consts_test.go
git commit -m "Scaffold Go module, docs, and model/effort constants"
```

---

### Task 2: Project settings read/write/restore

**Files:**
- Create: `settings.go`
- Test: `settings_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Disabled struct { Plugins []string; Skills []string }`
  - `func ReadDisabled(path string) (Disabled, error)` — parse `.claude/settings.json`; missing/invalid → empty `Disabled`, nil error
  - `func WriteDisabled(path string, allPlugins, disabledPlugins, allSkills, disabledSkills []string) error` — in-place update, preserving unrelated keys, removing empty maps
  - `func Restore(path string) error` — drop `enabledPlugins`/`skillOverrides`; delete file if it becomes empty

- [ ] **Step 1: Write the failing test**

`settings_test.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestReadDisabled(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, p, map[string]any{
		"enabledPlugins": map[string]any{"slack": false, "keepme": true},
		"skillOverrides": map[string]any{"caveman": "off", "other": "on"},
		"unrelated":      "keep",
	})
	d, err := ReadDisabled(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Plugins) != 1 || d.Plugins[0] != "slack" {
		t.Errorf("plugins = %v, want [slack]", d.Plugins)
	}
	if len(d.Skills) != 1 || d.Skills[0] != "caveman" {
		t.Errorf("skills = %v, want [caveman]", d.Skills)
	}
}

func TestReadDisabledMissing(t *testing.T) {
	d, err := ReadDisabled(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(d.Plugins) != 0 || len(d.Skills) != 0 {
		t.Errorf("missing file should yield empty Disabled, got %+v", d)
	}
}

func TestWriteDisabledPreservesAndPrunes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, p, map[string]any{
		"unrelated":      "keep",
		"enabledPlugins": map[string]any{"slack": false},
	})
	// slack now re-enabled (not in disabled); frontend disabled; skill caveman disabled.
	err := WriteDisabled(p, []string{"slack", "frontend"}, []string{"frontend"},
		[]string{"caveman"}, []string{"caveman"})
	if err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, p)
	if m["unrelated"] != "keep" {
		t.Errorf("unrelated key not preserved: %v", m)
	}
	ep := m["enabledPlugins"].(map[string]any)
	if _, ok := ep["slack"]; ok {
		t.Errorf("re-enabled slack should be pruned, got %v", ep)
	}
	if ep["frontend"] != false {
		t.Errorf("frontend should be false, got %v", ep["frontend"])
	}
	so := m["skillOverrides"].(map[string]any)
	if so["caveman"] != "off" {
		t.Errorf("caveman should be off, got %v", so["caveman"])
	}
}

func TestWriteDisabledRemovesEmptyMaps(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	// nothing disabled → enabledPlugins/skillOverrides should not appear
	if err := WriteDisabled(p, []string{"slack"}, nil, []string{"caveman"}, nil); err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, p)
	if _, ok := m["enabledPlugins"]; ok {
		t.Errorf("empty enabledPlugins should be omitted: %v", m)
	}
	if _, ok := m["skillOverrides"]; ok {
		t.Errorf("empty skillOverrides should be omitted: %v", m)
	}
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	writeJSON(t, p, map[string]any{
		"enabledPlugins": map[string]any{"slack": false},
		"skillOverrides": map[string]any{"caveman": "off"},
		"keep":           "yes",
	})
	if err := Restore(p); err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, p)
	if _, ok := m["enabledPlugins"]; ok {
		t.Errorf("enabledPlugins should be removed")
	}
	if m["keep"] != "yes" {
		t.Errorf("unrelated key should remain")
	}
}

func TestRestoreDeletesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	writeJSON(t, p, map[string]any{
		"enabledPlugins": map[string]any{"slack": false},
	})
	if err := Restore(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("file should be deleted when empty after restore")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'Disabled|Restore' -v`
Expected: FAIL — `undefined: ReadDisabled`.

- [ ] **Step 3: Write minimal implementation**

`settings.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"sort"
)

// Disabled holds the per-project disabled selections read from settings.json.
type Disabled struct {
	Plugins []string
	Skills  []string
}

// loadSettings reads path into a generic map. Missing or invalid JSON yields an
// empty map and nil error (matching the original script's tolerant behavior).
func loadSettings(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{}, nil
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func saveSettings(path string, m map[string]any) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// ReadDisabled returns the plugins disabled via enabledPlugins:false and the
// skills disabled via skillOverrides:"off".
func ReadDisabled(path string) (Disabled, error) {
	m, err := loadSettings(path)
	if err != nil {
		return Disabled{}, err
	}
	var d Disabled
	if ep, ok := m["enabledPlugins"].(map[string]any); ok {
		for name, v := range ep {
			if b, ok := v.(bool); ok && !b {
				d.Plugins = append(d.Plugins, name)
			}
		}
	}
	if so, ok := m["skillOverrides"].(map[string]any); ok {
		for name, v := range so {
			if s, ok := v.(string); ok && s == "off" {
				d.Skills = append(d.Skills, name)
			}
		}
	}
	sort.Strings(d.Plugins)
	sort.Strings(d.Skills)
	return d, nil
}

func contains(set []string, s string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// WriteDisabled updates settings.json in place. For each plugin in allPlugins it
// sets enabledPlugins[name]=false when disabled, else removes the key; same for
// skills via skillOverrides "off". Unrelated keys are preserved; empty maps are
// removed entirely.
func WriteDisabled(path string, allPlugins, disabledPlugins, allSkills, disabledSkills []string) error {
	m, err := loadSettings(path)
	if err != nil {
		return err
	}

	if len(allPlugins) > 0 {
		ep, _ := m["enabledPlugins"].(map[string]any)
		if ep == nil {
			ep = map[string]any{}
		}
		for _, p := range allPlugins {
			if contains(disabledPlugins, p) {
				ep[p] = false
			} else {
				delete(ep, p)
			}
		}
		if len(ep) > 0 {
			m["enabledPlugins"] = ep
		} else {
			delete(m, "enabledPlugins")
		}
	}

	if len(allSkills) > 0 {
		so, _ := m["skillOverrides"].(map[string]any)
		if so == nil {
			so = map[string]any{}
		}
		for _, s := range allSkills {
			if contains(disabledSkills, s) {
				so[s] = "off"
			} else {
				delete(so, s)
			}
		}
		if len(so) > 0 {
			m["skillOverrides"] = so
		} else {
			delete(m, "skillOverrides")
		}
	}

	return saveSettings(path, m)
}

// Restore removes the boot-managed keys. If the file becomes empty it is deleted.
func Restore(path string) error {
	m, err := loadSettings(path)
	if err != nil {
		return err
	}
	delete(m, "enabledPlugins")
	delete(m, "skillOverrides")
	if len(m) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return saveSettings(path, m)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add settings.go settings_test.go
git commit -m "Add project settings read/write/restore"
```

---

### Task 3: Discovery (plugins, skills, env defaults)

**Files:**
- Create: `discover.go`
- Test: `discover_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Item struct { Name string; Description string }`
  - `func DiscoverPlugins(jsonPath string) ([]Item, error)` — user-scoped plugin names from `installed_plugins.json`, deduped, sorted; description from the install object's `description` if present, else ""
  - `func DiscoverSkills(skillsDir string) ([]Item, error)` — subdirs (symlinks followed, dotfiles skipped), sorted; description from `SKILL.md` frontmatter
  - `func skillDescription(skillMdPath string) string` — parse the `description:` field from YAML frontmatter
  - `func EnvDefaults(getenv func(string) string) (model, effort string)` — current env values (getenv injected for testability)

- [ ] **Step 1: Write the failing test**

`discover_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPlugins(t *testing.T) {
	p := filepath.Join(t.TempDir(), "installed_plugins.json")
	os.WriteFile(p, []byte(`{
      "plugins": {
        "slack":       [{"scope": "user", "description": "Slack tools"}],
        "frontend":    [{"scope": "user"}],
        "projectonly": [{"scope": "project"}]
      }
    }`), 0o644)

	items, err := DiscoverPlugins(p)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, it := range items {
		names[it.Name] = it.Description
	}
	if _, ok := names["projectonly"]; ok {
		t.Errorf("project-scoped plugin should be excluded: %v", names)
	}
	if names["slack"] != "Slack tools" {
		t.Errorf("slack description = %q", names["slack"])
	}
	if _, ok := names["frontend"]; !ok {
		t.Errorf("frontend (no description) should still appear")
	}
}

func TestDiscoverPluginsMissing(t *testing.T) {
	items, err := DiscoverPlugins(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want empty, got %v", items)
	}
}

func TestDiscoverSkills(t *testing.T) {
	dir := t.TempDir()
	mk := func(name, frontmatter string) {
		sd := filepath.Join(dir, name)
		os.MkdirAll(sd, 0o755)
		os.WriteFile(filepath.Join(sd, "SKILL.md"), []byte(frontmatter), 0o644)
	}
	mk("artifactory", "---\nname: artifactory\ndescription: Search Artifactory\n---\nbody")
	mk("caveman", "---\ndescription: \"Caveman mode\"\n---\n")
	os.Mkdir(filepath.Join(dir, ".hidden"), 0o755) // must be skipped

	items, err := DiscoverSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.Name] = it.Description
	}
	if _, ok := got[".hidden"]; ok {
		t.Errorf("dotfile dir should be skipped")
	}
	if got["artifactory"] != "Search Artifactory" {
		t.Errorf("artifactory desc = %q", got["artifactory"])
	}
	if got["caveman"] != "Caveman mode" {
		t.Errorf("quoted desc not unwrapped: %q", got["caveman"])
	}
}

func TestEnvDefaults(t *testing.T) {
	env := map[string]string{EnvModel: "claude-opus-4-8", EnvEffort: "high"}
	m, e := EnvDefaults(func(k string) string { return env[k] })
	if m != "claude-opus-4-8" || e != "high" {
		t.Errorf("EnvDefaults = %q,%q", m, e)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'Discover|EnvDefaults' -v`
Expected: FAIL — `undefined: DiscoverPlugins`.

- [ ] **Step 3: Write minimal implementation**

`discover.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Item is a discovered plugin or skill.
type Item struct {
	Name        string
	Description string
}

// DiscoverPlugins returns user-scoped plugins from installed_plugins.json.
func DiscoverPlugins(jsonPath string) ([]Item, error) {
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var doc struct {
		Plugins map[string][]struct {
			Scope       string `json:"scope"`
			Description string `json:"description"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, nil // tolerant: malformed file → no plugins
	}
	var items []Item
	for name, installs := range doc.Plugins {
		for _, in := range installs {
			if in.Scope == "user" {
				items = append(items, Item{Name: name, Description: in.Description})
				break
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// DiscoverSkills lists immediate subdirectories of skillsDir (following symlinks,
// skipping dotfiles) and reads each SKILL.md frontmatter description.
func DiscoverSkills(skillsDir string) ([]Item, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var items []Item
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(skillsDir, name)
		info, err := os.Stat(full) // Stat follows symlinks
		if err != nil || !info.IsDir() {
			continue
		}
		desc := skillDescription(filepath.Join(full, "SKILL.md"))
		items = append(items, Item{Name: name, Description: desc})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// skillDescription extracts the `description:` value from a SKILL.md YAML
// frontmatter block. Returns "" if absent.
func skillDescription(skillMdPath string) string {
	b, err := os.ReadFile(skillMdPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	inFrontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break // end of frontmatter
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(trimmed, "description:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

// EnvDefaults reads the current model/effort env values via the injected getenv.
func EnvDefaults(getenv func(string) string) (model, effort string) {
	return getenv(EnvModel), getenv(EnvEffort)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add discover.go discover_test.go
git commit -m "Add discovery for plugins, skills, and env defaults"
```

---

### Task 4: Profiles store

**Files:**
- Create: `profiles.go`
- Test: `profiles_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Profile struct { Name string; Model string; Effort string; DisabledPlugins []string; DisabledSkills []string }` with JSON tags (`model,omitempty`, `effort,omitempty`)
  - `func LoadProfiles(path string) ([]Profile, error)` — missing file → empty slice, nil error
  - `func SaveProfile(path string, p Profile) error` — upsert by name, create parent dir, write file
  - `func FindProfile(profiles []Profile, name string) (Profile, bool)`
  - `func DefaultProfilesPath(home string) string` — `home/.claude/claude-boot/profiles.json`

- [ ] **Step 1: Write the failing test**

`profiles_test.go`:

```go
package main

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadProfiles(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "profiles.json") // parent dir must be created
	a := Profile{Name: "minimal", Model: "claude-haiku-4-5-20251001",
		DisabledPlugins: []string{"slack"}, DisabledSkills: []string{"caveman"}}
	if err := SaveProfile(p, a); err != nil {
		t.Fatal(err)
	}
	b := Profile{Name: "frontend", DisabledSkills: []string{"artifactory"}} // model/effort omitted
	if err := SaveProfile(p, b); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProfiles(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 profiles, got %d", len(got))
	}
	min, ok := FindProfile(got, "minimal")
	if !ok || min.Model != "claude-haiku-4-5-20251001" || len(min.DisabledPlugins) != 1 {
		t.Errorf("minimal profile wrong: %+v", min)
	}
	fe, _ := FindProfile(got, "frontend")
	if fe.Model != "" || fe.Effort != "" {
		t.Errorf("frontend should have empty model/effort: %+v", fe)
	}
}

func TestSaveProfileUpsert(t *testing.T) {
	p := filepath.Join(t.TempDir(), "profiles.json")
	SaveProfile(p, Profile{Name: "x", Effort: "low"})
	SaveProfile(p, Profile{Name: "x", Effort: "max"}) // overwrite same name
	got, _ := LoadProfiles(p)
	if len(got) != 1 {
		t.Fatalf("upsert should not duplicate, got %d", len(got))
	}
	if got[0].Effort != "max" {
		t.Errorf("upsert should replace, got %q", got[0].Effort)
	}
}

func TestLoadProfilesMissing(t *testing.T) {
	got, err := LoadProfiles(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

func TestDefaultProfilesPath(t *testing.T) {
	want := filepath.Join("/home/u", ".claude", "claude-boot", "profiles.json")
	if got := DefaultProfilesPath("/home/u"); got != want {
		t.Errorf("DefaultProfilesPath = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'Profile' -v`
Expected: FAIL — `undefined: SaveProfile`.

- [ ] **Step 3: Write minimal implementation**

`profiles.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Profile is a saved launch configuration. Model/Effort are optional.
type Profile struct {
	Name            string   `json:"name"`
	Model           string   `json:"model,omitempty"`
	Effort          string   `json:"effort,omitempty"`
	DisabledPlugins []string `json:"disabledPlugins"`
	DisabledSkills  []string `json:"disabledSkills"`
}

// DefaultProfilesPath returns the global profiles file location.
func DefaultProfilesPath(home string) string {
	return filepath.Join(home, ".claude", "claude-boot", "profiles.json")
}

// LoadProfiles reads the profiles file. A missing file yields an empty slice.
func LoadProfiles(path string) ([]Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ps []Profile
	if err := json.Unmarshal(b, &ps); err != nil {
		return nil, err
	}
	return ps, nil
}

// SaveProfile upserts p (by Name) and writes the file, creating the parent dir.
func SaveProfile(path string, p Profile) error {
	ps, err := LoadProfiles(path)
	if err != nil {
		return err
	}
	replaced := false
	for i := range ps {
		if ps[i].Name == p.Name {
			ps[i] = p
			replaced = true
			break
		}
	}
	if !replaced {
		ps = append(ps, p)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// FindProfile returns the named profile and whether it was found.
func FindProfile(profiles []Profile, name string) (Profile, bool) {
	for _, p := range profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add profiles.go profiles_test.go
git commit -m "Add global profiles store"
```

---

### Task 5: Child env + launch

**Files:**
- Create: `launch.go`
- Test: `launch_test.go`

**Interfaces:**
- Consumes: `EnvModel`, `EnvEffort` (Task 1).
- Produces:
  - `func ChildEnv(base []string, model, effort string) []string` — returns base with `ANTHROPIC_MODEL`/`CLAUDE_CODE_EFFORT_LEVEL` set when the corresponding value is non-empty; replaces any existing entry for that key
  - `func Launch(env []string) error` — `exec`s `claude` with env (thin wrapper around `syscall.Exec`; not unit-tested)

- [ ] **Step 1: Write the failing test**

`launch_test.go`:

```go
package main

import "testing"

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			return e[len(prefix):], true
		}
	}
	return "", false
}

func TestChildEnvSetsBoth(t *testing.T) {
	base := []string{"PATH=/bin", "ANTHROPIC_MODEL=old"}
	env := ChildEnv(base, "claude-opus-4-8", "high")
	if v, _ := envValue(env, EnvModel); v != "claude-opus-4-8" {
		t.Errorf("model = %q, want claude-opus-4-8 (should replace old)", v)
	}
	if v, _ := envValue(env, EnvEffort); v != "high" {
		t.Errorf("effort = %q", v)
	}
	// PATH preserved.
	if v, _ := envValue(env, "PATH"); v != "/bin" {
		t.Errorf("PATH not preserved: %q", v)
	}
}

func TestChildEnvInheritLeavesUnset(t *testing.T) {
	base := []string{"PATH=/bin"}
	env := ChildEnv(base, "", "") // both inherit
	if _, ok := envValue(env, EnvModel); ok {
		t.Errorf("inherit model should not set %s", EnvModel)
	}
	if _, ok := envValue(env, EnvEffort); ok {
		t.Errorf("inherit effort should not set %s", EnvEffort)
	}
}

func TestChildEnvNoDuplicateKey(t *testing.T) {
	base := []string{"ANTHROPIC_MODEL=old", "ANTHROPIC_MODEL=older"}
	env := ChildEnv(base, "claude-fable-5", "")
	count := 0
	for _, e := range env {
		if _, ok := envValue([]string{e}, EnvModel); ok {
			count++
		}
	}
	if count != 1 {
		t.Errorf("want exactly one %s entry, got %d", EnvModel, count)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'ChildEnv' -v`
Expected: FAIL — `undefined: ChildEnv`.

- [ ] **Step 3: Write minimal implementation**

`launch.go`:

```go
package main

import (
	"os/exec"
	"strings"
	"syscall"
)

// setEnv returns env with key=value, replacing any existing entries for key.
// If value is empty, key is left as-is (not added, not removed).
func setEnv(env []string, key, value string) []string {
	if value == "" {
		return env
	}
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return append(out, prefix+value)
}

// ChildEnv overlays model/effort onto base when each is non-empty.
func ChildEnv(base []string, model, effort string) []string {
	env := setEnv(base, EnvModel, model)
	env = setEnv(env, EnvEffort, effort)
	return env
}

// Launch replaces the current process with `claude`, passing env. Not unit-tested
// (it never returns on success).
func Launch(env []string) error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return err
	}
	return syscall.Exec(path, []string{"claude"}, env)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add launch.go launch_test.go
git commit -m "Add child env construction and claude exec"
```

---

### Task 6: TUI model + Update reducer

**Files:**
- Create: `tui.go`
- Test: `tui_test.go`
- Modify: `go.mod`, `go.sum` (add Bubble Tea deps)

**Interfaces:**
- Consumes: `Models`, `Efforts`, `ModelIndexByID`, `EffortIndex` (Task 1); `Item` (Task 3); `Profile`, `FindProfile` (Task 4).
- Produces:
  - `type rowKind int` with `kindPlugin`, `kindSkill`
  - `type row struct { kind rowKind; name, desc string; enabled bool }`
  - `type uiMode int` with `modeMain`, `modeProfilePick`, `modeProfileSave`
  - `type region int` with `regionModel`, `regionEffort`, `regionList`
  - `type model struct { ... }` (fields listed in impl)
  - `func newModel(plugins, skills []Item, disabled Disabled, curModel, curEffort string, profiles []Profile) model`
  - `func (m model) Init() tea.Cmd`
  - `func (m model) Update(tea.Msg) (tea.Model, tea.Cmd)`
  - `func (m *model) applyProfile(p Profile)`
  - `func (m model) visibleRows() []int` — indices of `m.rows` matching the current filter
  - `func (m model) result() (disabledPlugins, disabledSkills []string, modelID, effort string, launch bool)`
  - `func (m model) currentProfile(name string) Profile`
- Note: `View()` is added in Task 7; this task's `model` must compile, so include a minimal `func (m model) View() string { return "" }` placeholder to be replaced in Task 7.

- [ ] **Step 1: Add Bubble Tea dependencies**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles/textinput@latest
go get github.com/charmbracelet/lipgloss@latest
```

- [ ] **Step 2: Write the failing test**

`tui_test.go`:

```go
package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleModel() model {
	plugins := []Item{{Name: "slack"}, {Name: "frontend"}}
	skills := []Item{{Name: "artifactory"}, {Name: "caveman"}}
	disabled := Disabled{Plugins: []string{"frontend"}} // frontend starts disabled
	profiles := []Profile{{Name: "minimal", Model: "claude-haiku-4-5-20251001",
		Effort: "low", DisabledPlugins: []string{"slack"}, DisabledSkills: []string{"caveman"}}}
	return newModel(plugins, skills, disabled, "claude-opus-4-8", "high", profiles)
}

func send(m model, msg tea.Msg) model {
	res, _ := m.Update(msg)
	return res.(model)
}

func TestNewModelPrefillAndDisabled(t *testing.T) {
	m := sampleModel()
	if Models[m.modelIdx].ID != "claude-opus-4-8" {
		t.Errorf("model not prefilled: idx %d", m.modelIdx)
	}
	if Efforts[m.effortIdx] != "high" {
		t.Errorf("effort not prefilled: idx %d", m.effortIdx)
	}
	// frontend should start disabled (unchecked), slack enabled.
	for _, r := range m.rows {
		if r.name == "frontend" && r.enabled {
			t.Errorf("frontend should start disabled")
		}
		if r.name == "slack" && !r.enabled {
			t.Errorf("slack should start enabled")
		}
	}
}

func TestTabCyclesRegion(t *testing.T) {
	m := sampleModel()
	if m.region != regionModel {
		t.Fatalf("default region should be regionModel")
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.region != regionEffort {
		t.Errorf("after tab want regionEffort, got %d", m.region)
	}
	m = send(m, tea.KeyMsg{Type: tea.KeyTab})
	m = send(m, tea.KeyMsg{Type: tea.KeyTab}) // wraps back to model
	if m.region != regionModel {
		t.Errorf("region should wrap to regionModel, got %d", m.region)
	}
}

func TestModelEffortCycling(t *testing.T) {
	m := sampleModel() // region starts on model
	start := m.modelIdx
	m = send(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.modelIdx != (start+1)%len(Models) {
		t.Errorf("down should advance model idx")
	}
	m.region = regionEffort
	se := m.effortIdx
	m = send(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.effortIdx != (se-1+len(Efforts))%len(Efforts) {
		t.Errorf("up should decrement effort idx")
	}
}

func TestToggleListItem(t *testing.T) {
	m := sampleModel()
	m.region = regionList
	m.cursor = 0 // first visible row
	first := m.visibleRows()[0]
	before := m.rows[first].enabled
	m = send(m, tea.KeyMsg{Type: tea.KeySpace})
	if m.rows[first].enabled == before {
		t.Errorf("space should toggle enabled state")
	}
}

func TestFilterNarrowsRows(t *testing.T) {
	m := sampleModel()
	m.region = regionList
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // enter filter
	for _, r := range "cave" {
		m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	vis := m.visibleRows()
	if len(vis) != 1 || m.rows[vis[0]].name != "caveman" {
		t.Errorf("filter 'cave' should match only caveman, got %d rows", len(vis))
	}
}

func TestEnterLaunches(t *testing.T) {
	m := sampleModel()
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	_, _, _, _, launch := m.result()
	if !launch {
		t.Errorf("enter should set launch=true")
	}
}

func TestEscCancels(t *testing.T) {
	m := sampleModel()
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
	_, _, _, _, launch := m.result()
	if launch {
		t.Errorf("esc should not launch")
	}
	if !m.quit {
		t.Errorf("esc should set quit")
	}
}

func TestApplyProfile(t *testing.T) {
	m := sampleModel()
	p, _ := FindProfile(m.profiles, "minimal")
	m.applyProfile(p)
	if Models[m.modelIdx].ID != "claude-haiku-4-5-20251001" {
		t.Errorf("applyProfile model not set")
	}
	if Efforts[m.effortIdx] != "low" {
		t.Errorf("applyProfile effort not set")
	}
	for _, r := range m.rows {
		if r.name == "slack" && r.enabled {
			t.Errorf("profile disables slack; should be unchecked")
		}
		if r.name == "caveman" && r.enabled {
			t.Errorf("profile disables caveman; should be unchecked")
		}
	}
}

func TestResultCollectsDisabled(t *testing.T) {
	m := sampleModel() // frontend starts disabled
	dp, _, modelID, effort, _ := m.result()
	if len(dp) != 1 || dp[0] != "frontend" {
		t.Errorf("result disabled plugins = %v, want [frontend]", dp)
	}
	if modelID != "claude-opus-4-8" || effort != "high" {
		t.Errorf("result model/effort = %q/%q", modelID, effort)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./... -run 'TestNewModel|Region|Cycling|Toggle|Filter|Launch|Cancel|ApplyProfile|ResultCollects' -v`
Expected: FAIL — `undefined: newModel`.

- [ ] **Step 4: Write minimal implementation**

`tui.go`:

```go
package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type rowKind int

const (
	kindPlugin rowKind = iota
	kindSkill
)

type row struct {
	kind    rowKind
	name    string
	desc    string
	enabled bool
}

type region int

const (
	regionModel region = iota
	regionEffort
	regionList
)

type uiMode int

const (
	modeMain uiMode = iota
	modeProfilePick
	modeProfileSave
)

type model struct {
	rows      []row
	cursor    int // index into visibleRows()
	modelIdx  int
	effortIdx int
	region    region
	mode      uiMode

	filter    textinput.Model
	filtering bool

	profiles    []Profile
	profCursor  int
	saveName    textinput.Model
	savedNotice string

	quit   bool
	launch bool
}

func newModel(plugins, skills []Item, disabled Disabled, curModel, curEffort string, profiles []Profile) model {
	var rows []row
	for _, p := range plugins {
		rows = append(rows, row{kind: kindPlugin, name: p.Name, desc: p.Description,
			enabled: !contains(disabled.Plugins, p.Name)})
	}
	for _, s := range skills {
		rows = append(rows, row{kind: kindSkill, name: s.Name, desc: s.Description,
			enabled: !contains(disabled.Skills, s.Name)})
	}

	fi := textinput.New()
	fi.Placeholder = "filter…"
	si := textinput.New()
	si.Placeholder = "profile name"

	return model{
		rows:      rows,
		modelIdx:  ModelIndexByID(curModel),
		effortIdx: EffortIndex(curEffort),
		region:    regionModel,
		mode:      modeMain,
		filter:    fi,
		saveName:  si,
		profiles:  profiles,
	}
}

func (m model) Init() tea.Cmd { return nil }

// visibleRows returns the indices of m.rows matching the current filter text.
func (m model) visibleRows() []int {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	var out []int
	for i, r := range m.rows {
		if q == "" || strings.Contains(strings.ToLower(r.name), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m *model) clampCursor() {
	n := len(m.visibleRows())
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *model) toggle() {
	vis := m.visibleRows()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return
	}
	idx := vis[m.cursor]
	m.rows[idx].enabled = !m.rows[idx].enabled
}

func (m *model) applyProfile(p Profile) {
	m.modelIdx = ModelIndexByID(p.Model)
	m.effortIdx = EffortIndex(p.Effort)
	for i := range m.rows {
		switch m.rows[i].kind {
		case kindPlugin:
			m.rows[i].enabled = !contains(p.DisabledPlugins, m.rows[i].name)
		case kindSkill:
			m.rows[i].enabled = !contains(p.DisabledSkills, m.rows[i].name)
		}
	}
}

// currentProfile builds a Profile from the current screen state.
func (m model) currentProfile(name string) Profile {
	p := Profile{Name: name, Model: Models[m.modelIdx].ID, Effort: Efforts[m.effortIdx]}
	for _, r := range m.rows {
		if r.enabled {
			continue
		}
		if r.kind == kindPlugin {
			p.DisabledPlugins = append(p.DisabledPlugins, r.name)
		} else {
			p.DisabledSkills = append(p.DisabledSkills, r.name)
		}
	}
	return p
}

func (m model) result() (disabledPlugins, disabledSkills []string, modelID, effort string, launch bool) {
	for _, r := range m.rows {
		if r.enabled {
			continue
		}
		if r.kind == kindPlugin {
			disabledPlugins = append(disabledPlugins, r.name)
		} else {
			disabledSkills = append(disabledSkills, r.name)
		}
	}
	return disabledPlugins, disabledSkills, Models[m.modelIdx].ID, Efforts[m.effortIdx], m.launch
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.mode {
	case modeProfileSave:
		switch km.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(m.saveName.Value())
			if name != "" {
				m.profiles = upsertProfile(m.profiles, m.currentProfile(name))
				m.savedNotice = "saved profile: " + name
			}
			m.mode = modeMain
			m.saveName.Blur()
			m.saveName.SetValue("")
			return m, nil
		case tea.KeyEsc:
			m.mode = modeMain
			m.saveName.Blur()
			m.saveName.SetValue("")
			return m, nil
		default:
			var cmd tea.Cmd
			m.saveName, cmd = m.saveName.Update(km)
			return m, cmd
		}
	case modeProfilePick:
		switch km.Type {
		case tea.KeyUp:
			if m.profCursor > 0 {
				m.profCursor--
			}
		case tea.KeyDown:
			if m.profCursor < len(m.profiles)-1 {
				m.profCursor++
			}
		case tea.KeyEnter:
			if len(m.profiles) > 0 {
				m.applyProfile(m.profiles[m.profCursor])
			}
			m.mode = modeMain
		case tea.KeyEsc:
			m.mode = modeMain
		}
		return m, nil
	}

	// modeMain
	if m.filtering {
		switch km.Type {
		case tea.KeyEnter, tea.KeyEsc:
			m.filtering = false
			m.filter.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(km)
			m.clampCursor()
			return m, cmd
		}
	}

	switch km.Type {
	case tea.KeyTab:
		m.region = (m.region + 1) % 3
	case tea.KeyEsc:
		m.quit = true
		return m, tea.Quit
	case tea.KeyEnter:
		m.launch = true
		return m, tea.Quit
	case tea.KeyUp:
		m.move(-1)
	case tea.KeyDown:
		m.move(1)
	case tea.KeySpace:
		if m.region == regionList {
			m.toggle()
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "/":
			m.filtering = true
			m.filter.Focus()
		case "p":
			if len(m.profiles) > 0 {
				m.mode = modeProfilePick
				m.profCursor = 0
			}
		case "s":
			m.mode = modeProfileSave
			m.saveName.Focus()
		}
	}
	return m, nil
}

func (m *model) move(delta int) {
	switch m.region {
	case regionModel:
		m.modelIdx = (m.modelIdx + delta + len(Models)) % len(Models)
	case regionEffort:
		m.effortIdx = (m.effortIdx + delta + len(Efforts)) % len(Efforts)
	case regionList:
		m.cursor += delta
		m.clampCursor()
	}
}

func upsertProfile(ps []Profile, p Profile) []Profile {
	for i := range ps {
		if ps[i].Name == p.Name {
			ps[i] = p
			return ps
		}
	}
	return append(ps, p)
}

// View placeholder — replaced in Task 7.
func (m model) View() string { return "" }
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add tui.go tui_test.go go.mod go.sum
git commit -m "Add Bubble Tea model and Update reducer"
```

---

### Task 7: TUI View rendering

**Files:**
- Modify: `tui.go` (replace the `View()` placeholder; add Lipgloss styles)
- Test: `tui_test.go` (add view smoke tests)

**Interfaces:**
- Consumes: everything from Task 6.
- Produces: a complete `func (m model) View() string` that renders the model/effort fields, grouped list with checkboxes, detail pane, profile overlays, and footer.

- [ ] **Step 1: Write the failing test**

Add to `tui_test.go`:

```go
func TestViewContainsKeyElements(t *testing.T) {
	m := sampleModel()
	out := m.View()
	for _, want := range []string{"Model:", "Effort:", "Plugins", "Skills", "slack", "caveman", "launch"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q", want)
		}
	}
}

func TestViewShowsDisabledMarker(t *testing.T) {
	m := sampleModel() // frontend disabled
	out := m.View()
	// The detail pane should show the highlighted row's description region label.
	if !strings.Contains(out, "DETAIL") {
		t.Errorf("View() should contain a DETAIL pane label")
	}
	_ = out
}

func TestViewProfileSaveMode(t *testing.T) {
	m := sampleModel()
	m.mode = modeProfileSave
	out := m.View()
	if !strings.Contains(out, "Save profile") {
		t.Errorf("save mode view should prompt for a profile name")
	}
}
```

Add `"strings"` to the test imports if not already present (it is used above).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestView' -v`
Expected: FAIL — `View()` returns "" so the substrings are missing.

- [ ] **Step 3: Replace the View placeholder**

In `tui.go`, add the Lipgloss import and styles, and replace `func (m model) View() string { return "" }` with the implementation:

```go
// add to the import block:
//   "fmt"
//   "github.com/charmbracelet/lipgloss"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	activeStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	headerStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	detailStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(34)
	footerStyle   = lipgloss.NewStyle().Faint(true).Padding(1, 1, 0, 1)
	selectedRow   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	noticeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

func fieldLabel(active bool, label, value string) string {
	s := fmt.Sprintf("%s ▾ %s", label, value)
	if active {
		return activeStyle.Render(s)
	}
	return s
}

func (m model) View() string {
	if m.mode == modeProfileSave {
		return titleStyle.Render("claude-boot") + "\n\n" +
			"Save profile as:\n" + m.saveName.View() + "\n\n" +
			footerStyle.Render("enter save · esc cancel")
	}
	if m.mode == modeProfilePick {
		var b strings.Builder
		b.WriteString(titleStyle.Render("claude-boot") + "\n\n")
		b.WriteString(headerStyle.Render("Load profile") + "\n")
		for i, p := range m.profiles {
			cursor := "  "
			line := p.Name
			if i == m.profCursor {
				cursor = "▶ "
				line = selectedRow.Render(line)
			}
			b.WriteString(cursor + line + "\n")
		}
		b.WriteString("\n" + footerStyle.Render("↑/↓ choose · enter load · esc cancel"))
		return b.String()
	}

	// modeMain
	var b strings.Builder
	b.WriteString(titleStyle.Render("claude-boot") + "\n")

	modelVal := Models[m.modelIdx].Label
	effortLabel := Efforts[m.effortIdx]
	if effortLabel == "" {
		effortLabel = "— inherit (don't set)"
	}
	fields := fieldLabel(m.region == regionModel, "Model:", modelVal) +
		"    " + fieldLabel(m.region == regionEffort, "Effort:", effortLabel)
	b.WriteString(fields + "\n\n")

	// Build the list with section headers and a checkbox per row.
	vis := m.visibleRows()
	var listLines []string
	lastKind := rowKind(-1)
	plugins, skills := 0, 0
	for _, r := range m.rows {
		if r.kind == kindPlugin {
			plugins++
		} else {
			skills++
		}
	}
	highlighted := -1
	if m.region == regionList && m.cursor < len(vis) {
		highlighted = vis[m.cursor]
	}
	for _, idx := range vis {
		r := m.rows[idx]
		if r.kind != lastKind {
			if r.kind == kindPlugin {
				listLines = append(listLines, headerStyle.Render(fmt.Sprintf("Plugins (%d)", plugins)))
			} else {
				listLines = append(listLines, headerStyle.Render(fmt.Sprintf("Skills (%d)", skills)))
			}
			lastKind = r.kind
		}
		box := "✓"
		if !r.enabled {
			box = " "
		}
		pointer := "  "
		line := fmt.Sprintf("[%s] %s", box, r.name)
		if idx == highlighted {
			pointer = "▶ "
			line = selectedRow.Render(line)
		}
		listLines = append(listLines, pointer+line)
	}
	if m.filtering || m.filter.Value() != "" {
		listLines = append(listLines, "", m.filter.View())
	}
	listBlock := strings.Join(listLines, "\n")

	// Detail pane for the highlighted row.
	detail := "DETAIL\n\n(use ↑/↓ in the list)"
	if highlighted >= 0 {
		r := m.rows[highlighted]
		desc := r.desc
		if desc == "" {
			desc = "(no description)"
		}
		detail = fmt.Sprintf("DETAIL\n\n%s\n\n%s", r.name, desc)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(44).Render(listBlock), detailStyle.Render(detail))
	b.WriteString(body + "\n")

	footer := "space toggle · / filter · p profiles · s save · ↵ launch · esc cancel"
	b.WriteString(footerStyle.Render(footer))
	if m.savedNotice != "" {
		b.WriteString("\n" + noticeStyle.Render(m.savedNotice))
	}
	return b.String()
}
```

Update the `import` block at the top of `tui.go` to include `"fmt"` and `"github.com/charmbracelet/lipgloss"` alongside the existing imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Build and smoke-test interactively**

Run: `make build && ./claude-boot` (then press `esc` to cancel without launching).
Expected: the TUI renders model/effort fields, grouped Plugins/Skills list, detail pane, and footer; `esc` exits cleanly with no launch.

- [ ] **Step 6: Commit**

```bash
git add tui.go tui_test.go
git commit -m "Render TUI view: fields, grouped list, detail pane, overlays"
```

---

### Task 8: main.go wiring + arg parsing

**Files:**
- Create: `main.go`
- Test: `main_test.go`

**Interfaces:**
- Consumes: all prior tasks.
- Produces:
  - `type cliArgs struct { restore bool; profile string }`
  - `func parseArgs(argv []string) (cliArgs, error)` — recognizes `--restore` and `-p <name>`/`--profile <name>`
  - `func run(args cliArgs, env []string, home, cwd string) error` — orchestration seam usable from tests for the non-TUI paths (`--restore`, unknown `-p`); the interactive path is exercised manually
  - `func main()` — reads real env/home/cwd, calls `run`

- [ ] **Step 1: Write the failing test**

`main_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseArgs(t *testing.T) {
	a, err := parseArgs([]string{"--restore"})
	if err != nil || !a.restore {
		t.Errorf("--restore not parsed: %+v %v", a, err)
	}
	a, err = parseArgs([]string{"-p", "minimal"})
	if err != nil || a.profile != "minimal" {
		t.Errorf("-p not parsed: %+v %v", a, err)
	}
	a, err = parseArgs([]string{"--profile", "frontend"})
	if err != nil || a.profile != "frontend" {
		t.Errorf("--profile not parsed: %+v %v", a, err)
	}
	if _, err := parseArgs([]string{"-p"}); err == nil {
		t.Errorf("-p without value should error")
	}
}

func TestRunRestore(t *testing.T) {
	cwd := t.TempDir()
	claudeDir := filepath.Join(cwd, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	settings := filepath.Join(claudeDir, "settings.json")
	os.WriteFile(settings, []byte(`{"enabledPlugins":{"slack":false}}`), 0o644)

	err := run(cliArgs{restore: true}, os.Environ(), t.TempDir(), cwd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Errorf("restore should have deleted the now-empty settings file")
	}
}

func TestRunUnknownProfile(t *testing.T) {
	home := t.TempDir() // no profiles file
	err := run(cliArgs{profile: "nope"}, os.Environ(), home, t.TempDir())
	if err == nil {
		t.Errorf("unknown profile should return an error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'ParseArgs|RunRestore|RunUnknownProfile' -v`
Expected: FAIL — `undefined: parseArgs`.

- [ ] **Step 3: Write minimal implementation**

`main.go`:

```go
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
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	if err := run(args, os.Environ(), home, cwd); err != nil {
		fmt.Fprintln(os.Stderr, "claude-boot:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Build and verify the full binary**

Run: `make build && go vet ./...`
Expected: builds cleanly, vet reports nothing.

Manual smoke checks:
- `./claude-boot --restore` in a dir with a `.claude/settings.json` → prints the restore message.
- `./claude-boot -p nonexistent` → prints `profile not found: nonexistent`, exits non-zero.
- `./claude-boot` → TUI launches; toggling, filtering, `s`/`p`, and `esc` all behave.

- [ ] **Step 6: Commit**

```bash
git add main.go main_test.go
git commit -m "Wire arg parsing and orchestration; complete claude-boot"
```

---

## Self-Review

**Spec coverage:**
- Stack/location/build → Task 1 (go.mod, Makefile, CLAUDE.md, README symlink) ✓
- Grouped view + checkboxes → Task 7 view (Plugins/Skills headers) ✓
- Live filter → Task 6 reducer (`/`, `visibleRows`) + Task 7 render ✓
- Detail pane → Task 7 ✓
- Profiles (everything, model/effort optional) → Task 4 (store) + Task 6 (apply/save) + Task 8 (quick-launch) ✓
- Model/effort bootstrap, prefill, inherit sentinel → Task 1 (consts) + Task 5 (ChildEnv) + Task 6 (prefill) ✓
- Single screen + quick-launch → Task 7 (single screen) + Task 8 (`-p` skips TUI) ✓
- Persistence semantics table → Task 2 (settings) + Task 5 (child-env only) + Task 4 (global profiles) ✓
- `--restore` preserved → Task 2 + Task 8 ✓
- Discovery sources → Task 3 ✓
- Error handling (tolerant parsing, unknown profile, direct launch) → Tasks 2/3/8 ✓
- Testing strategy → unit tests in every task; reducer tested directly ✓
- Drop fzf dependency → no fzf anywhere; native TUI ✓
- No PATH wiring / scripts/bin untouched → not in scope of any task ✓

**Placeholder scan:** The only intentional placeholder is `View()` in Task 6, explicitly replaced in Task 7 (called out in both tasks). No "TBD"/"add error handling"/"similar to Task N" left.

**Type consistency:** `Item`, `Disabled`, `Profile`, `model`, `row`, `region`, `uiMode`, `ModelChoice` and the function signatures (`ReadDisabled`/`WriteDisabled`/`Restore`, `DiscoverPlugins`/`DiscoverSkills`, `LoadProfiles`/`SaveProfile`/`FindProfile`/`DefaultProfilesPath`, `ChildEnv`/`Launch`, `newModel`/`applyProfile`/`result`/`visibleRows`, `parseArgs`/`run`/`names`) are used consistently across tasks. `contains` (Task 2) is reused in Tasks 6. `upsertProfile` (Task 6) mirrors `SaveProfile`'s upsert for the in-memory list.
```
