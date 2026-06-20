package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleModel() model {
	plugins := []Item{{Name: "slack"}, {Name: "frontend"}}
	skills := []Item{{Name: "artifactory"}, {Name: "caveman"}}
	disabled := Disabled{Plugins: []string{"frontend"}} // frontend starts disabled
	profiles := []Profile{{Name: "minimal", Model: "claude-haiku-4-5-20251001",
		Effort: "low", DisabledPlugins: []string{"slack"}, DisabledSkills: []string{"caveman"}}}
	return newModel(plugins, skills, disabled, "claude-opus-4-8", "high", profiles, "/home/user/.claude", "/home/user")
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

func TestSaveProfileUpsertViaReducer(t *testing.T) {
	m := sampleModel() // has profile "minimal"; frontend starts disabled
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.mode != modeProfileSave {
		t.Fatalf("expected modeProfileSave, got %d", m.mode)
	}
	m.saveName.SetValue("minimal") // existing name
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeMain {
		t.Errorf("save should return to modeMain, got %d", m.mode)
	}
	count := 0
	for _, p := range m.profiles {
		if p.Name == "minimal" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("upsert should not duplicate 'minimal', got %d", count)
	}
	p, ok := FindProfile(m.profiles, "minimal")
	if !ok {
		t.Fatal("minimal profile missing after save")
	}
	if !contains(p.DisabledPlugins, "frontend") {
		t.Errorf("saved profile should capture frontend as disabled, got %v", p.DisabledPlugins)
	}
}

func TestSaveProfileAppendsNewName(t *testing.T) {
	m := sampleModel()
	before := len(m.profiles)
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m.saveName.SetValue("brand-new")
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.profiles) != before+1 {
		t.Errorf("new name should append; want %d, got %d", before+1, len(m.profiles))
	}
	if _, ok := FindProfile(m.profiles, "brand-new"); !ok {
		t.Errorf("appended profile not found")
	}
}

func TestProfilePickEmptyStaysMain(t *testing.T) {
	m := newModel([]Item{{Name: "slack"}}, nil, Disabled{}, "", "", nil, "/home/user/.claude", "/home/user") // no profiles
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if m.mode != modeMain {
		t.Errorf("'p' with no profiles should stay modeMain, got %d", m.mode)
	}
}

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
	m := sampleModel() // frontend starts disabled, slack enabled; region=regionModel (no row highlighted)
	out := m.View()
	if !strings.Contains(out, "DETAIL") {
		t.Errorf("View() should contain a DETAIL pane label")
	}
	if !strings.Contains(out, "[✓] slack") {
		t.Errorf("enabled plugin slack should render a checked box; out:\n%s", out)
	}
	if !strings.Contains(out, "[ ] frontend") {
		t.Errorf("disabled plugin frontend should render an empty box; out:\n%s", out)
	}
}

func TestViewProfileSaveMode(t *testing.T) {
	m := sampleModel()
	m.mode = modeProfileSave
	out := m.View()
	if !strings.Contains(out, "Save profile") {
		t.Errorf("save mode view should prompt for a profile name")
	}
}

func TestViewProfilePickMode(t *testing.T) {
	m := sampleModel() // has profile "minimal"
	m.mode = modeProfilePick
	out := m.View()
	if !strings.Contains(out, "Load profile") {
		t.Errorf("pick mode should show 'Load profile' header; out:\n%s", out)
	}
	if !strings.Contains(out, "minimal") {
		t.Errorf("pick mode should list the 'minimal' profile; out:\n%s", out)
	}
}

func TestSelectAllNone(t *testing.T) {
	m := sampleModel() // frontend disabled, rest enabled
	// 'n' disables all visible rows
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	for _, r := range m.rows {
		if r.enabled {
			t.Errorf("after 'n', row %q should be disabled", r.name)
		}
	}
	// 'a' enables all visible rows
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	for _, r := range m.rows {
		if !r.enabled {
			t.Errorf("after 'a', row %q should be enabled", r.name)
		}
	}
}

func TestViewShowsConfigDir(t *testing.T) {
	m := sampleModel() // configDir set to "~/.claude" via sampleModel
	out := m.View()
	if !strings.Contains(out, "config:") {
		t.Errorf("View() should show config dir; out:\n%s", out)
	}
}
