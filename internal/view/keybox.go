package view

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pain/internal/theme"
)

// keysHint ends every footer. The footers only show the keys that matter
// where you are; ctrl+/ lists them all.
const KeysHint = "ctrl+/ keys"

type KeyRow struct{ Key, Action string }

func KeysBox(w, h int, title string, rows []KeyRow) string {
	kw := 0
	for _, r := range rows {
		kw = max(kw, lipgloss.Width(r.Key))
	}
	lines := []string{lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render(title), ""}
	for _, r := range rows {
		pad := strings.Repeat(" ", kw-lipgloss.Width(r.Key)+2)
		lines = append(lines, theme.Fg(theme.Yellow).Render(r.Key)+pad+theme.Fg(theme.Subtext1).Render(r.Action))
	}
	lines = append(lines, "", theme.Fg(theme.Overlay0).Render("any key to close"))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Overlay0)).Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
