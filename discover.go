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
			if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
}

// EnvDefaults reads the current model/effort env values via the injected getenv.
func EnvDefaults(getenv func(string) string) (model, effort string) {
	return getenv(EnvModel), getenv(EnvEffort)
}

// ClaudeConfigDir returns the effective Claude config directory: $CLAUDE_CONFIG_DIR
// when set, otherwise ~/.claude.
func ClaudeConfigDir(getenv func(string) string, home string) string {
	if v := getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return v
	}
	return filepath.Join(home, ".claude")
}
