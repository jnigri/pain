package reader

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pain/internal/store"
	"pain/internal/theme"
)

// prefItems drives the preferences screen.
var prefItems = []struct {
	name, desc string
	field      func(*store.Prefs) *bool
}{
	{"typing mode", "enter types the next space instead of skipping the selection", func(p *store.Prefs) *bool { return &p.Typing }},
	{"explain at checkpoints", "active reading: explain each section yourself instead of answering questions", func(p *store.Prefs) *bool { return &p.Explain }},
}

// The model row comes after the switches and opens the model picker.
var (
	modelRow = len(prefItems)
	prefRows = len(prefItems) + 1
)

func (m *Model) prefsView() string {
	title := theme.Fg(theme.Purple).Bold(true).Render("preferences")
	var rows []string
	for i, it := range prefItems {
		box := theme.Fg(theme.Muted).Render("[ ]")
		if *it.field(&m.prefs) {
			box = theme.Fg(theme.Green).Render("[✔]")
		}
		name := theme.Fg(theme.Foreground).Render(it.name)
		mark := "  "
		if i == m.prefSel {
			mark = theme.Fg(theme.Yellow).Render("› ")
			name = theme.Fg(theme.Foreground).Bold(true).Render(it.name)
		}
		rows = append(rows, mark+box+" "+name, "      "+theme.Fg(theme.Muted).Render(it.desc))
	}
	name := theme.Fg(theme.Foreground).Render("model")
	mark := "  "
	if m.prefSel == modelRow {
		mark = theme.Fg(theme.Yellow).Render("› ")
		name = theme.Fg(theme.Foreground).Bold(true).Render("model")
	}
	rows = append(rows, mark+theme.Fg(theme.Blue).Render("[…]")+" "+name+"  "+theme.Fg(theme.Yellow).Render(modelName(m.prefs.Model)),
		"      "+theme.Fg(theme.Muted).Render("the Claude model that answers and writes the checkpoints"))
	help := theme.Fg(theme.Muted).Render("↑/↓ move · space toggle · enter choose · esc close")
	foot := theme.Fg(theme.Muted).Render(strings.Replace(store.PrefsPath(), os.Getenv("HOME"), "~", 1))
	if m.prefErr != nil {
		foot = theme.Fg(theme.Red).Render("could not save: " + m.prefErr.Error())
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Edge()).Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, append(append([]string{title, ""}, rows...), "", help, foot)...))
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, box)
}
