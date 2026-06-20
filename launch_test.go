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
