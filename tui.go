package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type rowKind int

const (
	kindPlugin rowKind = iota
	kindSkill
)

type row struct {
	kind    rowKind
	name    string
	desc    string
	enabled bool
}

type region int

const (
	regionModel region = iota
	regionEffort
	regionList
)

type uiMode int

const (
	modeMain uiMode = iota
	modeProfilePick
	modeProfileSave
)

type model struct {
	rows      []row
	cursor    int // index into visibleRows()
	modelIdx  int
	effortIdx int
	region    region
	mode      uiMode

	filter    textinput.Model
	filtering bool

	profiles    []Profile
	profCursor  int
	saveName    textinput.Model
	savedNotice string

	configDir string // display only

	quit   bool
	launch bool
}

func newModel(plugins, skills []Item, disabled Disabled, curModel, curEffort string, profiles []Profile, configDir, home string) model {
	var rows []row
	for _, p := range plugins {
		rows = append(rows, row{kind: kindPlugin, name: p.Name, desc: p.Description,
			enabled: !contains(disabled.Plugins, p.Name)})
	}
	for _, s := range skills {
		rows = append(rows, row{kind: kindSkill, name: s.Name, desc: s.Description,
			enabled: !contains(disabled.Skills, s.Name)})
	}

	fi := textinput.New()
	fi.Placeholder = "filter…"
	si := textinput.New()
	si.Placeholder = "profile name"

	displayDir := configDir
	if home != "" && strings.HasPrefix(configDir, home) {
		displayDir = "~" + configDir[len(home):]
	}

	return model{
		rows:      rows,
		modelIdx:  ModelIndexByID(curModel),
		effortIdx: EffortIndex(curEffort),
		region:    regionModel,
		mode:      modeMain,
		filter:    fi,
		saveName:  si,
		profiles:  profiles,
		configDir: displayDir,
	}
}

func (m model) Init() tea.Cmd { return nil }

// visibleRows returns the indices of m.rows matching the current filter text.
func (m model) visibleRows() []int {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	var out []int
	for i, r := range m.rows {
		if q == "" || strings.Contains(strings.ToLower(r.name), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m *model) clampCursor() {
	n := len(m.visibleRows())
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *model) toggle() {
	vis := m.visibleRows()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return
	}
	idx := vis[m.cursor]
	m.rows[idx].enabled = !m.rows[idx].enabled
}

func (m *model) applyProfile(p Profile) {
	m.modelIdx = ModelIndexByID(p.Model)
	m.effortIdx = EffortIndex(p.Effort)
	for i := range m.rows {
		switch m.rows[i].kind {
		case kindPlugin:
			m.rows[i].enabled = !contains(p.DisabledPlugins, m.rows[i].name)
		case kindSkill:
			m.rows[i].enabled = !contains(p.DisabledSkills, m.rows[i].name)
		}
	}
}

// currentProfile builds a Profile from the current screen state.
func (m model) currentProfile(name string) Profile {
	p := Profile{Name: name, Model: Models[m.modelIdx].ID, Effort: Efforts[m.effortIdx]}
	for _, r := range m.rows {
		if r.enabled {
			continue
		}
		if r.kind == kindPlugin {
			p.DisabledPlugins = append(p.DisabledPlugins, r.name)
		} else {
			p.DisabledSkills = append(p.DisabledSkills, r.name)
		}
	}
	return p
}

func (m model) result() (disabledPlugins, disabledSkills []string, modelID, effort string, launch bool) {
	for _, r := range m.rows {
		if r.enabled {
			continue
		}
		if r.kind == kindPlugin {
			disabledPlugins = append(disabledPlugins, r.name)
		} else {
			disabledSkills = append(disabledSkills, r.name)
		}
	}
	return disabledPlugins, disabledSkills, Models[m.modelIdx].ID, Efforts[m.effortIdx], m.launch
}

// Update is the reducer. NOTE: it has a value receiver and mutates the local
// copy `m` via pointer-receiver helpers (move/clampCursor/toggle). Those
// mutations reach the caller only through the final `return m, nil`; any early
// return added to the modeMain handling MUST return the mutated `m`.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.mode {
	case modeProfileSave:
		switch km.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(m.saveName.Value())
			if name != "" {
				m.profiles = upsertProfile(m.profiles, m.currentProfile(name))
				m.savedNotice = "saved profile: " + name
			}
			m.mode = modeMain
			m.saveName.Blur()
			m.saveName.SetValue("")
			return m, nil
		case tea.KeyEsc:
			m.mode = modeMain
			m.saveName.Blur()
			m.saveName.SetValue("")
			return m, nil
		default:
			var cmd tea.Cmd
			m.saveName, cmd = m.saveName.Update(km)
			return m, cmd
		}
	case modeProfilePick:
		switch km.Type {
		case tea.KeyUp:
			if m.profCursor > 0 {
				m.profCursor--
			}
		case tea.KeyDown:
			if m.profCursor < len(m.profiles)-1 {
				m.profCursor++
			}
		case tea.KeyEnter:
			if len(m.profiles) > 0 {
				m.applyProfile(m.profiles[m.profCursor])
			}
			m.mode = modeMain
		case tea.KeyEsc:
			m.mode = modeMain
		}
		return m, nil
	}

	// modeMain
	if m.filtering {
		switch km.Type {
		case tea.KeyEnter, tea.KeyEsc:
			m.filtering = false
			m.filter.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(km)
			m.clampCursor()
			return m, cmd
		}
	}

	switch km.Type {
	case tea.KeyTab:
		m.region = (m.region + 1) % 3
	case tea.KeyEsc:
		m.quit = true
		return m, tea.Quit
	case tea.KeyEnter:
		m.launch = true
		return m, tea.Quit
	case tea.KeyUp:
		m.move(-1)
	case tea.KeyDown:
		m.move(1)
	case tea.KeySpace:
		if m.region == regionList {
			m.toggle()
		}
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "/":
			m.filtering = true
			m.filter.Focus()
		case "p":
			if len(m.profiles) > 0 {
				m.mode = modeProfilePick
				m.profCursor = 0
			}
		case "s":
			m.mode = modeProfileSave
			m.saveName.Focus()
		case "a":
			for _, idx := range m.visibleRows() {
				m.rows[idx].enabled = true
			}
		case "n":
			for _, idx := range m.visibleRows() {
				m.rows[idx].enabled = false
			}
		}
	}
	return m, nil
}

func (m *model) move(delta int) {
	switch m.region {
	case regionModel:
		m.modelIdx = (m.modelIdx + delta + len(Models)) % len(Models)
	case regionEffort:
		m.effortIdx = (m.effortIdx + delta + len(Efforts)) % len(Efforts)
	case regionList:
		m.cursor += delta
		m.clampCursor()
	}
}

func upsertProfile(ps []Profile, p Profile) []Profile {
	for i := range ps {
		if ps[i].Name == p.Name {
			ps[i] = p
			return ps
		}
	}
	return append(ps, p)
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	activeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	headerStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	detailStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(34)
	footerStyle = lipgloss.NewStyle().Faint(true).Padding(1, 1, 0, 1)
	selectedRow = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	noticeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

func fieldLabel(active bool, label, value string) string {
	s := fmt.Sprintf("%s ▾ %s", label, value)
	if active {
		return activeStyle.Render(s)
	}
	return s
}

func (m model) View() string {
	if m.mode == modeProfileSave {
		return titleStyle.Render("claude-boot") + "\n\n" +
			"Save profile as:\n" + m.saveName.View() + "\n\n" +
			footerStyle.Render("enter save · esc cancel")
	}
	if m.mode == modeProfilePick {
		var b strings.Builder
		b.WriteString(titleStyle.Render("claude-boot") + "\n\n")
		b.WriteString(headerStyle.Render("Load profile") + "\n")
		for i, p := range m.profiles {
			cursor := "  "
			line := p.Name
			if i == m.profCursor {
				cursor = "▶ "
				line = selectedRow.Render(line)
			}
			b.WriteString(cursor + line + "\n")
		}
		b.WriteString("\n" + footerStyle.Render("↑/↓ choose · enter load · esc cancel"))
		return b.String()
	}

	// modeMain
	var b strings.Builder
	b.WriteString(titleStyle.Render("claude-boot") + "\n")
	if m.configDir != "" {
		b.WriteString(footerStyle.Render("config: "+m.configDir) + "\n")
	}

	modelVal := Models[m.modelIdx].Label
	effortLabel := Efforts[m.effortIdx]
	if effortLabel == "" {
		effortLabel = "— inherit (don't set)"
	}
	fields := fieldLabel(m.region == regionModel, "Model:", modelVal) +
		"    " + fieldLabel(m.region == regionEffort, "Effort:", effortLabel)
	b.WriteString(fields + "\n\n")

	// Build the list with section headers and a checkbox per row.
	vis := m.visibleRows()
	var listLines []string
	lastKind := rowKind(-1)
	plugins, skills := 0, 0
	for _, r := range m.rows {
		if r.kind == kindPlugin {
			plugins++
		} else {
			skills++
		}
	}
	highlighted := -1
	if m.region == regionList && m.cursor < len(vis) {
		highlighted = vis[m.cursor]
	}
	for _, idx := range vis {
		r := m.rows[idx]
		if r.kind != lastKind {
			if r.kind == kindPlugin {
				listLines = append(listLines, headerStyle.Render(fmt.Sprintf("Plugins (%d)", plugins)))
			} else {
				listLines = append(listLines, headerStyle.Render(fmt.Sprintf("Skills (%d)", skills)))
			}
			lastKind = r.kind
		}
		box := "✓"
		if !r.enabled {
			box = " "
		}
		pointer := "  "
		line := fmt.Sprintf("[%s] %s", box, r.name)
		if idx == highlighted {
			pointer = "▶ "
			line = selectedRow.Render(line)
		}
		listLines = append(listLines, pointer+line)
	}
	if m.filtering || m.filter.Value() != "" {
		listLines = append(listLines, "", m.filter.View())
	}
	listBlock := strings.Join(listLines, "\n")

	// Detail pane for the highlighted row.
	detail := "DETAIL\n\n(use ↑/↓ in the list)"
	if highlighted >= 0 {
		r := m.rows[highlighted]
		desc := r.desc
		if desc == "" {
			desc = "(no description)"
		}
		detail = fmt.Sprintf("DETAIL\n\n%s\n\n%s", r.name, desc)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(44).Render(listBlock), detailStyle.Render(detail))
	b.WriteString(body + "\n")

	footer := "space toggle · a all · n none · / filter · p profiles · s save · ↵ launch · esc cancel"
	b.WriteString(footerStyle.Render(footer))
	if m.savedNotice != "" {
		b.WriteString("\n" + noticeStyle.Render(m.savedNotice))
	}
	return b.String()
}
