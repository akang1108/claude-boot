package main

const (
	EnvModel  = "ANTHROPIC_MODEL"
	EnvEffort = "CLAUDE_CODE_EFFORT_LEVEL"
)

// ModelChoice is one entry in the model picker.
type ModelChoice struct {
	Label string // friendly label shown in the TUI
	ID    string // ANTHROPIC_MODEL value; "" means "inherit / don't set"
}

// Models is the picker lineup. Index 0 is the inherit sentinel.
var Models = []ModelChoice{
	{Label: "— inherit (don't set)", ID: ""},
	{Label: "Opus 4.8", ID: "claude-opus-4-8"},
	{Label: "Sonnet 4.6", ID: "claude-sonnet-4-6"},
	{Label: "Haiku 4.5", ID: "claude-haiku-4-5-20251001"},
	{Label: "Fable 5", ID: "claude-fable-5"},
}

// Efforts is the CLAUDE_CODE_EFFORT_LEVEL ladder. Index 0 is the inherit sentinel.
var Efforts = []string{"", "low", "medium", "high", "xhigh", "max"}

// ModelIndexByID returns the Models index whose ID matches id, or 0 (inherit).
func ModelIndexByID(id string) int {
	for i, m := range Models {
		if m.ID == id && id != "" {
			return i
		}
	}
	return 0
}

// EffortIndex returns the Efforts index matching v, or 0 (inherit).
func EffortIndex(v string) int {
	for i, e := range Efforts {
		if e == v && v != "" {
			return i
		}
	}
	return 0
}
