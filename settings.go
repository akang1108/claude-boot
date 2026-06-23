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

// ReadGlobalDefaults returns the model and effortLevel from the global
// ~/.claude/settings.json (top-level keys, not the claudeBoot namespace).
func ReadGlobalDefaults(path string) (model, effort string) {
	m, err := loadSettings(path)
	if err != nil {
		return "", ""
	}
	model, _ = m["model"].(string)
	effort, _ = m["effortLevel"].(string)
	return model, effort
}

// ReadModelEffort returns the model and effort stored under the "claudeBoot"
// namespace in settings.json. Either may be "" if not set.
func ReadModelEffort(path string) (model, effort string, err error) {
	m, loadErr := loadSettings(path)
	if loadErr != nil {
		return "", "", loadErr
	}
	cb, _ := m["claudeBoot"].(map[string]any)
	if cb != nil {
		model, _ = cb["model"].(string)
		effort, _ = cb["effort"].(string)
	}
	return model, effort, nil
}

// WriteModelEffort persists model/effort under "claudeBoot" in settings.json.
// When both are empty (inherit), the key is removed entirely.
func WriteModelEffort(path, model, effort string) error {
	m, err := loadSettings(path)
	if err != nil {
		return err
	}
	if model == "" && effort == "" {
		delete(m, "claudeBoot")
	} else {
		cb := map[string]any{}
		if model != "" {
			cb["model"] = model
		}
		if effort != "" {
			cb["effort"] = effort
		}
		m["claudeBoot"] = cb
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
	delete(m, "claudeBoot")
	if len(m) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return saveSettings(path, m)
}
