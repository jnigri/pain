package hold

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pain/ui/story"
)

// Stories show a hold confirming a choice in a list, the way pain answers
// a question.
var Stories = []story.Story{
	{
		Name:  "hold to answer",
		About: "hold enter or a number; the choice underlines as you hold",
		New:   func() tea.Model { return newChoices(time.Second) },
	},
	{
		Name:  "hold to answer, slow",
		About: "the same over three seconds, to watch the underline and letting go",
		New:   func() tea.Model { return newChoices(3 * time.Second) },
	},
}

type choices struct {
	hold   Hold
	items  []string
	sel    int
	picked int // -1 before
	log    []string
}

func newChoices(d time.Duration) *choices {
	return &choices{
		hold:   New(d),
		picked: -1,
		items: []string{
			"Yes",
			"A choice long enough to show the underline running under its words",
			"No, not at all",
		},
	}
}

func (m *choices) Init() tea.Cmd { return nil }

func (m *choices) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if done, cmd := m.hold.Update(msg); done {
		m.picked = m.sel
		m.note("answered %d", m.sel+1)
	} else if cmd != nil {
		return m, cmd
	} else if _, ok := msg.(TickMsg); ok && !m.hold.Running() {
		m.note("let go")
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	key := k.String()
	if mine, cmd := m.hold.Repeat(key); mine {
		return m, cmd
	}
	n := len(m.items)
	switch {
	case key == "esc" && m.hold.Running():
		m.hold.Cancel()
		m.note("taken back")
	case key == "r":
		m.picked = -1
	case m.picked >= 0:
	case key == "up" || key == "k":
		m.hold.Cancel()
		m.sel = (m.sel + n - 1) % n
	case key == "down" || key == "j":
		m.hold.Cancel()
		m.sel = (m.sel + 1) % n
	case key == "enter":
		m.note("holding %s", key)
		return m, m.hold.Start(key)
	case len(key) == 1 && key[0] >= '1' && int(key[0]-'1') < n:
		m.sel = int(key[0] - '1')
		m.note("holding %s", key)
		return m, m.hold.Start(key)
	}
	return m, nil
}

func (m *choices) note(f string, a ...any) {
	m.log = append(m.log, time.Now().Format("15:04:05.000")+"  "+fmt.Sprintf(f, a...))
	if len(m.log) > 6 {
		m.log = m.log[1:]
	}
}

// hint says how to answer, by the choice selected.
func (m *choices) hint() string {
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	switch {
	case m.hold.Running():
		return yellow.Render("keep holding")
	case m.hold.Tapped():
		return yellow.Render("hold the key to answer")
	}
	return lipgloss.NewStyle().Faint(true).Render("hold to answer")
}

func (m *choices) View() string {
	const width = 60
	dim := lipgloss.NewStyle().Faint(true)
	bold := lipgloss.NewStyle().Bold(true)
	var sb strings.Builder
	sb.WriteString(bold.Render("Which one?") + "\n\n")
	for i, it := range m.items {
		mark, st := "  ", lipgloss.NewStyle()
		switch {
		case m.picked >= 0 && i == m.picked:
			mark, st = "✔ ", st.Foreground(lipgloss.Color("2"))
		case m.picked < 0 && i == m.sel:
			mark, st = "› ", bold
		}
		lead := mark + dim.Render(fmt.Sprintf("%d  ", i+1))
		for k, l := range strings.Split(st.Width(width-5).Render(it), "\n") {
			line := lead + l
			if m.picked < 0 && i == m.sel && k == 0 {
				line = Hint(line, width, m.hint())
			}
			if m.hold.Running() && i == m.sel {
				line = Underline(line, 5, width, m.hold.Progress(), DefaultLook)
			}
			sb.WriteString(line + "\n")
			lead = "     "
		}
	}
	help := "↑/↓ move · hold enter or 1–3 to answer · esc take back"
	if m.picked >= 0 {
		help = "r again"
	}
	sb.WriteString("\n" + dim.Render(help) + "\n\n")
	sb.WriteString(dim.Render(strings.Join(m.log, "\n")))
	return sb.String()
}
