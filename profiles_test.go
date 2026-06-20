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
	if err := SaveProfile(p, Profile{Name: "x", Effort: "low"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfile(p, Profile{Name: "x", Effort: "max"}); err != nil { // overwrite same name
		t.Fatal(err)
	}
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
	if got := DefaultProfilesPath("/home/u/.claude"); got != want {
		t.Errorf("DefaultProfilesPath = %q, want %q", got, want)
	}
}
