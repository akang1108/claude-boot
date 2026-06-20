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
