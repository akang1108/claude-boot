package main

import "testing"

func TestModelsLineup(t *testing.T) {
	if Models[0].ID != "" {
		t.Fatalf("index 0 must be inherit sentinel, got %q", Models[0].ID)
	}
	want := map[string]string{
		"Opus 4.8":   "claude-opus-4-8",
		"Sonnet 4.6": "claude-sonnet-4-6",
		"Haiku 4.5":  "claude-haiku-4-5-20251001",
		"Fable 5":    "claude-fable-5",
	}
	got := map[string]string{}
	for _, m := range Models[1:] {
		got[m.Label] = m.ID
	}
	for label, id := range want {
		if got[label] != id {
			t.Errorf("model %q: want id %q, got %q", label, id, got[label])
		}
	}
}

func TestEffortsLadder(t *testing.T) {
	if Efforts[0] != "" {
		t.Fatalf("index 0 must be inherit sentinel, got %q", Efforts[0])
	}
	want := []string{"low", "medium", "high", "xhigh", "max"}
	for i, v := range want {
		if Efforts[i+1] != v {
			t.Errorf("effort %d: want %q, got %q", i+1, v, Efforts[i+1])
		}
	}
}

func TestModelIndexByID(t *testing.T) {
	if got := ModelIndexByID("claude-sonnet-4-6"); Models[got].ID != "claude-sonnet-4-6" {
		t.Errorf("ModelIndexByID returned %d", got)
	}
	if got := ModelIndexByID("nonexistent"); got != 0 {
		t.Errorf("unknown id should map to 0, got %d", got)
	}
}

func TestEffortIndex(t *testing.T) {
	if got := EffortIndex("high"); Efforts[got] != "high" {
		t.Errorf("EffortIndex returned %d", got)
	}
	if got := EffortIndex(""); got != 0 {
		t.Errorf("empty should map to 0, got %d", got)
	}
}
