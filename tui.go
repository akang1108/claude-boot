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
	regionPlugins region = iota
	regionSkills
	regionModel
	regionEffort
)

type uiMode int

const (
	modeMain uiMode = iota
	modeProfilePick
	modeProfileSave
)

type model struct {
	rows         []row
	pluginCursor int
	skillCursor  int
	modelIdx     int
	effortIdx    int
	region       region
	mode         uiMode

	filter    textinput.Model
	filtering bool

	profiles    []Profile
	profCursor  int
	saveName    textinput.Model
	savedNotice string

	configDir    string // display only
	globalModel  string // from ~/.claude/settings.json, shown in inherit label
	globalEffort string // from ~/.claude/settings.json, shown in inherit label

	width      int
	height     int
	scrollTop  int // first vis-cursor index shown in combined list
	listHeight int // available lines for the list section (0 = unlimited)

	quit   bool
	launch bool
}

func newModel(plugins, skills []Item, disabled Disabled, curModel, curEffort string, profiles []Profile, configDir, home, globalModel, globalEffort string) model {
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
		region:    regionPlugins,
		mode:      modeMain,
		filter:    fi,
		saveName:  si,
		profiles:     profiles,
		configDir:    displayDir,
		globalModel:  globalModel,
		globalEffort: globalEffort,
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

func (m model) visiblePluginRows() []int {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	var out []int
	for i, r := range m.rows {
		if r.kind == kindPlugin && (q == "" || strings.Contains(strings.ToLower(r.name), q)) {
			out = append(out, i)
		}
	}
	return out
}

func (m model) visibleSkillRows() []int {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	var out []int
	for i, r := range m.rows {
		if r.kind == kindSkill && (q == "" || strings.Contains(strings.ToLower(r.name), q)) {
			out = append(out, i)
		}
	}
	return out
}

// activeCursorRowIndex returns the m.rows index of the currently highlighted item, or -1.
func (m model) activeCursorRowIndex() int {
	switch m.region {
	case regionPlugins:
		pvis := m.visiblePluginRows()
		if m.pluginCursor >= 0 && m.pluginCursor < len(pvis) {
			return pvis[m.pluginCursor]
		}
	case regionSkills:
		svis := m.visibleSkillRows()
		if m.skillCursor >= 0 && m.skillCursor < len(svis) {
			return svis[m.skillCursor]
		}
	}
	return -1
}

// overheadLines estimates lines consumed by the header and footer sections.
func (m model) overheadLines() int {
	n := 8 // title(1) + pickers(2) + footer(3) + buffer(2)
	if m.configDir != "" {
		n += 2 // config line + footerStyle top-padding
	}
	return n
}

// ensureVisible adjusts scrollTop so the active cursor row stays within the visible window.
func (m *model) ensureVisible() {
	if m.listHeight <= 0 {
		return
	}
	avail := m.listHeight - 2 // reserve 2 lines for section headers
	if avail < 1 {
		avail = 1
	}
	vis := m.visibleRows()
	n := len(vis)
	if n == 0 {
		m.scrollTop = 0
		return
	}
	targetRowIdx := m.activeCursorRowIndex()
	if targetRowIdx < 0 {
		return
	}
	targetPos := -1
	for i, idx := range vis {
		if idx == targetRowIdx {
			targetPos = i
			break
		}
	}
	if targetPos < 0 {
		return
	}
	if targetPos < m.scrollTop {
		m.scrollTop = targetPos
	} else if targetPos >= m.scrollTop+avail {
		m.scrollTop = targetPos - avail + 1
	}
	maxTop := n - avail
	if maxTop < 0 {
		maxTop = 0
	}
	if m.scrollTop > maxTop {
		m.scrollTop = maxTop
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
}

func (m *model) clampCursor() {
	pvis := m.visiblePluginRows()
	if len(pvis) == 0 {
		m.pluginCursor = 0
	} else if m.pluginCursor >= len(pvis) {
		m.pluginCursor = len(pvis) - 1
	} else if m.pluginCursor < 0 {
		m.pluginCursor = 0
	}

	svis := m.visibleSkillRows()
	if len(svis) == 0 {
		m.skillCursor = 0
	} else if m.skillCursor >= len(svis) {
		m.skillCursor = len(svis) - 1
	} else if m.skillCursor < 0 {
		m.skillCursor = 0
	}

	m.ensureVisible()
}

func (m *model) toggle() {
	switch m.region {
	case regionPlugins:
		pvis := m.visiblePluginRows()
		if m.pluginCursor < 0 || m.pluginCursor >= len(pvis) {
			return
		}
		idx := pvis[m.pluginCursor]
		m.rows[idx].enabled = !m.rows[idx].enabled
	case regionSkills:
		svis := m.visibleSkillRows()
		if m.skillCursor < 0 || m.skillCursor >= len(svis) {
			return
		}
		idx := svis[m.skillCursor]
		m.rows[idx].enabled = !m.rows[idx].enabled
	}
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
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = sz.Width
		m.height = sz.Height
		if sz.Height > 0 {
			m.listHeight = sz.Height - m.overheadLines()
			if m.listHeight < 3 {
				m.listHeight = 3
			}
		}
		m.ensureVisible()
		return m, nil
	}
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
		case tea.KeyRunes:
			if string(km.Runes) == "d" && len(m.profiles) > 0 {
				m.profiles = append(m.profiles[:m.profCursor], m.profiles[m.profCursor+1:]...)
				if m.profCursor >= len(m.profiles) && m.profCursor > 0 {
					m.profCursor--
				}
				if len(m.profiles) == 0 {
					m.mode = modeMain
				}
			}
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
		m.region = (m.region + 1) % 4
		m.ensureVisible()
	case tea.KeyShiftTab:
		m.region = (m.region - 1 + 4) % 4
		m.ensureVisible()
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
		if m.region == regionPlugins || m.region == regionSkills {
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
	case regionPlugins:
		pvis := m.visiblePluginRows()
		if len(pvis) == 0 {
			return
		}
		m.pluginCursor += delta
		if m.pluginCursor < 0 {
			m.pluginCursor = 0
		}
		if m.pluginCursor >= len(pvis) {
			m.pluginCursor = len(pvis) - 1
		}
		m.ensureVisible()
	case regionSkills:
		svis := m.visibleSkillRows()
		if len(svis) == 0 {
			return
		}
		m.skillCursor += delta
		if m.skillCursor < 0 {
			m.skillCursor = 0
		}
		if m.skillCursor >= len(svis) {
			m.skillCursor = len(svis) - 1
		}
		m.ensureVisible()
	case regionModel:
		m.modelIdx = (m.modelIdx + delta + len(Models)) % len(Models)
	case regionEffort:
		m.effortIdx = (m.effortIdx + delta + len(Efforts)) % len(Efforts)
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
	titleStyle        = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	activeStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	headerStyle       = lipgloss.NewStyle().Bold(true).Underline(true)
	activeSectionStyle = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("212"))
	footerStyle       = lipgloss.NewStyle().Faint(true).Padding(1, 1, 0, 1)
	selectedRow       = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	noticeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

// truncateLine shortens s to fit within max visible columns, appending "…" if cut.
func truncateLine(s string, max int) string {
	if max <= 0 {
		return s
	}
	w := lipgloss.Width(s)
	if w <= max {
		return s
	}
	// strip runes until we fit, accounting for the ellipsis
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > max {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

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
		b.WriteString("\n" + footerStyle.Render("↑/↓ choose · enter load · d delete · esc cancel"))
		return b.String()
	}

	// modeMain
	var b strings.Builder
	b.WriteString(titleStyle.Render("claude-boot") + "\n")

	w := m.width
	if w <= 0 {
		w = 80 // safe default before first WindowSizeMsg
	}

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
	highlighted := m.activeCursorRowIndex()
	// prefix is 2 cols ("  " or "▶ "), checkbox+space is 4 cols ("[✓] "), so name budget = w - 6
	nameBudget := w - 6
	if nameBudget < 10 {
		nameBudget = 10
	}

	// Determine which slice of vis to render.
	visStart := 0
	visEnd := len(vis)
	if m.listHeight > 0 && len(vis) > 0 {
		avail := m.listHeight - 2 // reserve 2 lines for section headers
		if avail < 1 {
			avail = 1
		}
		visStart = m.scrollTop
		visEnd = visStart + avail
		if visEnd > len(vis) {
			visEnd = len(vis)
		}
	}

	if visStart > 0 {
		listLines = append(listLines, fmt.Sprintf("  ↑ %d more", visStart))
	}
	for i := visStart; i < visEnd; i++ {
		idx := vis[i]
		r := m.rows[idx]
		if r.kind != lastKind {
			if r.kind == kindPlugin {
				hdr := fmt.Sprintf("Plugins (%d)", plugins)
				if m.region == regionPlugins {
					listLines = append(listLines, activeSectionStyle.Render(hdr))
				} else {
					listLines = append(listLines, headerStyle.Render(hdr))
				}
			} else {
				hdr := fmt.Sprintf("Skills (%d)", skills)
				if m.region == regionSkills {
					listLines = append(listLines, activeSectionStyle.Render(hdr))
				} else {
					listLines = append(listLines, headerStyle.Render(hdr))
				}
			}
			lastKind = r.kind
		}
		box := "✓"
		if !r.enabled {
			box = " "
		}
		pointer := "  "
		name := truncateLine(r.name, nameBudget)
		line := fmt.Sprintf("[%s] %s", box, name)
		if idx == highlighted {
			pointer = "▶ "
			line = selectedRow.Render(line)
		}
		listLines = append(listLines, pointer+line)
	}
	if visEnd < len(vis) {
		listLines = append(listLines, fmt.Sprintf("  ↓ %d more", len(vis)-visEnd))
	}

	if m.filtering || m.filter.Value() != "" {
		listLines = append(listLines, "", m.filter.View())
	}
	b.WriteString(strings.Join(listLines, "\n") + "\n")

	// Model/Effort pickers at bottom
	modelVal := Models[m.modelIdx].Label
	if m.modelIdx == 0 {
		if m.globalModel != "" {
			modelVal = "— inherit (global: " + m.globalModel + ")"
		} else {
			modelVal = "— inherit (don't set)"
		}
	}
	effortLabel := Efforts[m.effortIdx]
	if effortLabel == "" {
		if m.globalEffort != "" {
			effortLabel = "— inherit (global: " + m.globalEffort + ")"
		} else {
			effortLabel = "— inherit (don't set)"
		}
	}
	f1 := fieldLabel(m.region == regionModel, "Model:", modelVal)
	f2 := fieldLabel(m.region == regionEffort, "Effort:", effortLabel)
	if w < 60 {
		b.WriteString("\n" + f1 + "\n" + f2 + "\n")
	} else {
		b.WriteString("\n" + f1 + "    " + f2 + "\n")
	}

	var footer string
	if w < 60 {
		footer = "tab/⇧tab sections · space toggle · a all · n none · / filter\np profiles · s save · ↵ launch · esc quit"
	} else {
		footer = "tab/⇧tab sections · space toggle · a all · n none · / filter · p profiles · s save · ↵ launch · esc quit"
	}
	if m.configDir != "" {
		b.WriteString(footerStyle.Render("config: "+m.configDir) + "\n")
	}
	b.WriteString(footerStyle.Render(footer))
	if m.savedNotice != "" {
		b.WriteString("\n" + noticeStyle.Render(m.savedNotice))
	}
	return b.String()
}
