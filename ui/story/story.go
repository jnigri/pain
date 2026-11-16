// Package story runs UI components on their own, one screen each, to try
// them out away from the app that uses them, like stories in a web UI
// workshop. Each component package lists its stories; cmd/stories runs them
// all.
package story

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Story is one way to show a component.
type Story struct {
	Name  string
	About string
	New   func() tea.Model // a fresh screen each time the story opens
}

// Run lists the stories and shows the one picked. ctrl+o comes back to the
// list, ctrl+c quits. With a single story, it opens straight away.
func Run(stories []Story) error {
	m := &runner{stories: stories}
	if len(stories) == 1 {
		m.open(0)
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type runner struct {
	stories []Story
	sel     int
	cur     tea.Model // the story shown, nil on the list
	w, h    int
}

func (m *runner) open(i int) tea.Cmd {
	m.cur = m.stories[i].New()
	cmd := m.cur.Init()
	if m.w > 0 {
		var c tea.Cmd
		m.cur, c = m.cur.Update(tea.WindowSizeMsg{Width: m.w, Height: m.h})
		cmd = tea.Batch(cmd, c)
	}
	return cmd
}

func (m *runner) Init() tea.Cmd {
	if m.cur != nil {
		return m.cur.Init()
	}
	return nil
}

func (m *runner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.w, m.h = ws.Width, ws.Height
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+o":
			m.cur = nil
			return m, nil
		}
	}
	if m.cur != nil {
		var cmd tea.Cmd
		m.cur, cmd = m.cur.Update(msg)
		return m, cmd
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "up", "k":
			m.sel = (m.sel + len(m.stories) - 1) % len(m.stories)
		case "down", "j":
			m.sel = (m.sel + 1) % len(m.stories)
		case "enter":
			return m, m.open(m.sel)
		case "q", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

var (
	dim  = lipgloss.NewStyle().Faint(true)
	bold = lipgloss.NewStyle().Bold(true)
)

func (m *runner) View() string {
	if m.cur != nil {
		return m.cur.View() + "\n\n" + dim.Render("ctrl+o stories · ctrl+c quit")
	}
	var sb strings.Builder
	sb.WriteString(bold.Render("stories") + "\n\n")
	for i, s := range m.stories {
		mark, name := "  ", s.Name
		if i == m.sel {
			mark, name = "› ", bold.Render(name)
		}
		sb.WriteString(mark + name + "\n")
		if s.About != "" {
			sb.WriteString("  " + dim.Render(s.About) + "\n")
		}
	}
	sb.WriteString("\n" + dim.Render("↑/↓ move · enter open · q quit"))
	return sb.String()
}
