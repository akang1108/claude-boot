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

// DefaultProfilesPath returns the profiles file location within configDir.
func DefaultProfilesPath(configDir string) string {
	return filepath.Join(configDir, "claude-boot", "profiles.json")
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
