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
	if _, err := parseArgs([]string{"--profile"}); err == nil {
		t.Errorf("--profile without value should error")
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
