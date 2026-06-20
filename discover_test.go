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

func TestClaudeConfigDir(t *testing.T) {
	home := "/home/user"
	// env var set → use it
	got := ClaudeConfigDir(func(k string) string {
		if k == "CLAUDE_CONFIG_DIR" {
			return "/custom/config"
		}
		return ""
	}, home)
	if got != "/custom/config" {
		t.Errorf("got %q, want /custom/config", got)
	}
	// env var not set → ~/.claude
	got = ClaudeConfigDir(func(string) string { return "" }, home)
	if got != "/home/user/.claude" {
		t.Errorf("got %q, want /home/user/.claude", got)
	}
}

func TestEnvDefaults(t *testing.T) {
	env := map[string]string{EnvModel: "claude-opus-4-8", EnvEffort: "high"}
	m, e := EnvDefaults(func(k string) string { return env[k] })
	if m != "claude-opus-4-8" || e != "high" {
		t.Errorf("EnvDefaults = %q,%q", m, e)
	}
}

func TestSkillDescriptionMatchedQuotes(t *testing.T) {
	dir := t.TempDir()
	sd := filepath.Join(dir, "x")
	os.MkdirAll(sd, 0o755)
	os.WriteFile(filepath.Join(sd, "SKILL.md"),
		[]byte("---\ndescription: \"ends with a quote\\\"\"\n---\n"), 0o644)
	items, err := DiscoverSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	// Outer matched quotes removed; escaped quote in YAML value preserved.
	if items[0].Description != "ends with a quote\\\"" {
		t.Errorf("description = %q, want %q", items[0].Description, "ends with a quote\\\"")
	}
}

func TestDiscoverSkillsMissing(t *testing.T) {
	items, err := DiscoverSkills(filepath.Join(t.TempDir(), "no-such-skills-dir"))
	if err != nil {
		t.Fatalf("missing skills dir should not error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want empty, got %v", items)
	}
}
