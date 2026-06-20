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

func TestReadModelEffort(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, p, map[string]any{
		"claudeBoot": map[string]any{"model": "claude-opus-4-8", "effort": "high"},
		"unrelated":  "keep",
	})
	model, effort, err := ReadModelEffort(p)
	if err != nil {
		t.Fatal(err)
	}
	if model != "claude-opus-4-8" {
		t.Errorf("model = %q, want claude-opus-4-8", model)
	}
	if effort != "high" {
		t.Errorf("effort = %q, want high", effort)
	}
}

func TestReadModelEffortMissing(t *testing.T) {
	model, effort, err := ReadModelEffort(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if model != "" || effort != "" {
		t.Errorf("missing file should yield empty strings, got %q %q", model, effort)
	}
}

func TestWriteModelEffort(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	writeJSON(t, p, map[string]any{"unrelated": "keep"})

	if err := WriteModelEffort(p, "claude-sonnet-4-6", "medium"); err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, p)
	cb := m["claudeBoot"].(map[string]any)
	if cb["model"] != "claude-sonnet-4-6" {
		t.Errorf("model = %v, want claude-sonnet-4-6", cb["model"])
	}
	if cb["effort"] != "medium" {
		t.Errorf("effort = %v, want medium", cb["effort"])
	}
	if m["unrelated"] != "keep" {
		t.Errorf("unrelated key should be preserved")
	}

	// Reset to inherit → key removed.
	if err := WriteModelEffort(p, "", ""); err != nil {
		t.Fatal(err)
	}
	m = readJSON(t, p)
	if _, ok := m["claudeBoot"]; ok {
		t.Errorf("claudeBoot should be removed when both are empty")
	}
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	writeJSON(t, p, map[string]any{
		"enabledPlugins": map[string]any{"slack": false},
		"skillOverrides": map[string]any{"caveman": "off"},
		"claudeBoot":     map[string]any{"model": "claude-opus-4-8"},
		"keep":           "yes",
	})
	if err := Restore(p); err != nil {
		t.Fatal(err)
	}
	m := readJSON(t, p)
	if _, ok := m["enabledPlugins"]; ok {
		t.Errorf("enabledPlugins should be removed")
	}
	if _, ok := m["claudeBoot"]; ok {
		t.Errorf("claudeBoot should be removed")
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
