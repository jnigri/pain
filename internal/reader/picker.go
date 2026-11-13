package reader

import (
	"github.com/charmbracelet/lipgloss"

	"pain/internal/store"
	"pain/internal/theme"
)

// modelChoices are the models offered for asking Claude. The claude CLI has
// no command to list models, so these are its aliases, which always point to
// the latest model of each family. Any other name can still be set by hand in
// config.json.
var modelChoices = []struct{ id, name string }{
	{store.ModelDefault, "Claude Code default"},
	{"fable", "Fable"},
	{"opus", "Opus"},
	{"sonnet", "Sonnet"},
	{"haiku", "Haiku"},
}

// picker is the model choice screen. It opens on the first question, before
// a model has been chosen, and from the preferences.
type picker struct {
	ids     []string
	sel     int
	thenAsk bool   // chosen on the way to asking Claude
	then    func() // what to carry on with once a model is chosen
}

func newPicker(current string, thenAsk bool) *picker {
	p := &picker{thenAsk: thenAsk}
	for _, c := range modelChoices {
		p.ids = append(p.ids, c.id)
	}
	if current != "" && modelName(current) == current {
		p.ids = append(p.ids, current) // a name set by hand in config.json
	}
	for i, id := range p.ids {
		if id == current {
			p.sel = i
		}
	}
	return p
}

// modelName is how a model setting reads on screen.
func modelName(id string) string {
	if id == "" {
		id = store.ModelDefault
	}
	for _, c := range modelChoices {
		if c.id == id {
			return c.name
		}
	}
	return id
}

func (m *Model) updatePicker(key string) {
	p := m.pick
	switch key {
	case "esc", "q", "ctrl+c":
		m.pick = nil
	case "up", "k", "shift+tab":
		p.sel = (p.sel + len(p.ids) - 1) % len(p.ids)
	case "down", "j", "tab":
		p.sel = (p.sel + 1) % len(p.ids)
	case " ", "enter":
		m.prefs.Model = p.ids[p.sel]
		m.prefErr = m.prefs.Save()
		m.pick = nil
		if p.then != nil {
			p.then()
		}
	}
}

func (m *Model) pickerView() string {
	p := m.pick
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render("model")
	lines := []string{title, theme.Fg(theme.Subtext0).Render("answers your questions and writes the checkpoints"), ""}
	for i, id := range p.ids {
		mark := "  "
		name := theme.Fg(theme.Subtext1).Render(modelName(id))
		if i == p.sel {
			mark = theme.Fg(theme.Yellow).Render("› ")
			name = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Foreground)).Bold(true).Render(modelName(id))
		}
		hint := "uses your claude settings"
		if id != store.ModelDefault {
			hint = "--model " + id
		}
		lines = append(lines, mark+name+"  "+theme.Fg(theme.Overlay2).Render(hint))
	}
	lines = append(lines, "", theme.Fg(theme.Overlay0).Render("↑/↓ move · enter choose · esc cancel"))
	if p.thenAsk {
		lines = append(lines, theme.Fg(theme.Overlay0).Render("change it later with ctrl+p"))
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Overlay0)).Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, box)
}
