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
	{"case sensitive", "a and A count as different letters", func(p *store.Prefs) *bool { return &p.CaseSensitive }},
	{"skip punctuation", "typing the next letter fills in the punctuation before it", func(p *store.Prefs) *bool { return &p.SkipPunct }},
	{"split ask view", "show questions and answers below the text instead of full screen", func(p *store.Prefs) *bool { return &p.SplitAsk }},
}

func (m *Model) prefsView() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render("preferences")
	var rows []string
	for i, it := range prefItems {
		box := theme.Fg(theme.Overlay2).Render("[ ]")
		if *it.field(&m.prefs) {
			box = theme.Fg(theme.Green).Render("[✔]")
		}
		name := theme.Fg(theme.Subtext1).Render(it.name)
		mark := "  "
		if i == m.prefSel {
			mark = theme.Fg(theme.Yellow).Render("› ")
			name = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Foreground)).Bold(true).Render(it.name)
		}
		rows = append(rows, mark+box+" "+name, "      "+theme.Fg(theme.Overlay2).Render(it.desc))
	}
	help := theme.Fg(theme.Overlay0).Render("↑/↓ move · space toggle · esc close")
	foot := theme.Fg(theme.Overlay0).Render(strings.Replace(store.PrefsPath(), os.Getenv("HOME"), "~", 1))
	if m.prefErr != nil {
		foot = theme.Fg(theme.Red).Render("could not save: " + m.prefErr.Error())
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Overlay0)).Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, append(append([]string{title, ""}, rows...), "", help, foot)...))
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, box)
}
