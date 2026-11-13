package reader

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pain/internal/markdown"
	"pain/internal/theme"
	"pain/internal/view"
)

func (m *Model) askView() string {
	cw := m.contentWidth()
	margin := strings.Repeat(" ", max((m.w-cw)/2, 0))
	label := "ask"
	switch {
	case m.ask.review && m.ask.sec >= 0:
		label = "checkpoint · explain"
	case m.ask.review:
		label = "explain"
	}
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render(label)
	name := theme.Fg(theme.Subtext0).Render(filepath.Base(m.Path))
	gap := max(cw-lipgloss.Width(title)-lipgloss.Width(name), 1)
	var sb strings.Builder
	sb.WriteString(margin + title + strings.Repeat(" ", gap) + name + "\n\n")
	for i, l := range m.askPane(cw, max(m.h-2, 3)) {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(margin + l)
	}
	return sb.String()
}

// rule is a horizontal line with a label in the middle.
func rule(w int, label string) string {
	label = " " + label + " "
	lw := lipgloss.Width(label)
	side := max(w-lw, 2) / 2
	return strings.Repeat("─", side) + label + strings.Repeat("─", max(w-side-lw, 1))
}

// textLines renders plain text wrapped to w, through the normal unit drawing.
// A prefix is drawn in the ordered-list color.
func textLines(s string, w int, prefix string, quote int) []string {
	u := markdown.Unit{Kind: markdown.Para, Cells: markdown.Cells(s, 0, false), Prefix: prefix, Ordered: true, Quote: quote}
	lines, _ := view.RenderUnit(&u, w, nil, true)
	return lines
}

func markdownLines(s string, w int) []string {
	var out []string
	for _, u := range markdown.Parse([]byte(s)) {
		lines, _ := view.RenderUnit(&u, w, nil, true)
		out = append(out, lines...)
	}
	return out
}

// askPane draws the ask view in exactly h lines, the last one being help.
func (m *Model) askPane(cw, h int) []string {
	a := m.ask
	if a.mode == askHistory {
		return m.historyPane(cw, h)
	}

	// The block or sentence, drawn as in the document when it is still there.
	var top []string
	i := -1
	if !a.review {
		i = m.askedUnit(a.entry)
	}
	var sel []markdown.Cell
	if i >= 0 && a.entry.Selection != "" {
		sel = selectionCells(&m.units[i], a.entry.Selection)
	}
	switch {
	case a.review:
		top = wrapped(theme.Fg(theme.Subtext0), a.revTitle, cw, "")
		// What to do shows until the first explanation is sent.
		if len(a.entry.Turns) == 0 && a.mode == askInput {
			top = append(top, "", lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Foreground)).Bold(true).
				Render("Explain it in your own words, from memory."))
			top = append(top, wrapped(theme.Fg(theme.Overlay2),
				"A few sentences are enough. Claude says whether you got it or what you missed, and you can talk it over.", cw, "")...)
		}
	case a.entry.Section:
		top = textLines(m.askTitle(a.entry), cw, "", 1)
	case sel != nil:
		u := m.units[i]
		u.Quote++
		u.Cells, u.Prefix = sel, ""
		top, _ = view.RenderUnit(&u, cw, nil, true)
	case i >= 0 && a.entry.Selection == "":
		u := m.units[i]
		u.Quote++
		top, _ = view.RenderUnit(&u, cw, nil, true)
	case a.entry.Selection != "":
		top = textLines(a.entry.Selection, cw, "", 1)
	default:
		top = textLines(a.entry.Block, cw, "", 1)
	}
	if limit := max(h/4, 1); len(top) > limit {
		top = append(top[:limit-1], theme.Fg(theme.Overlay2).Render("┃ …"))
	}
	top = append(top, "")

	var body []string
	for i, t := range a.entry.Turns {
		if a.anchor == i+1 {
			a.scroll, a.anchor = len(body), 0
		}
		body = append(body, textLines(t.Question, cw, "› ", 0)...)
		body = append(body, imageLines(t.Images)...)
		body = append(body, "")
		body = append(body, markdownLines(t.Answer, cw)...)
		body = append(body, "")
	}
	switch a.mode {
	case askStream:
		body = append(body, textLines(a.pending, cw, "› ", 0)...)
		body = append(body, imageLines(a.pendImg)...)
		body = append(body, "")
		if a.partial == "" {
			body = append(body, theme.Fg(theme.Overlay2).Render("thinking…"))
		} else {
			body = append(body, markdownLines(a.partial, cw)...)
		}
	case askInput:
		if a.err != "" {
			body = append(body, strings.Split(theme.Fg(theme.Red).Width(cw).Render(a.err), "\n")...)
			body = append(body, "")
		}
		a.input.SetWidth(cw)
		ql := strings.Split(a.input.View(), "\n")
		body = append(body, ql[:min(a.input.LineInfo().Height, len(ql))]...)
		body = append(body, imageLines(a.images)...)
	case askAnswer:
		for len(body) > 0 && body[len(body)-1] == "" {
			body = body[:len(body)-1]
		}
	}

	bodyH := max(h-len(top)-2, 1)
	if len(top) > h-3 {
		top = top[:max(h-3, 0)]
		bodyH = 1
	}
	total := len(body)
	maxScroll := max(total-bodyH, 0)
	if a.follow {
		a.scroll = maxScroll
	}
	a.scroll = min(a.scroll, maxScroll)
	if a.mode != askAnswer && a.scroll == maxScroll {
		a.follow = true
	}
	body = body[a.scroll:min(a.scroll+bodyH, len(body))]
	for len(body) < bodyH {
		body = append(body, "")
	}

	var help string
	switch a.mode {
	case askInput:
		help = "enter send · esc back"
		if a.canWiden() {
			to := "the whole block"
			switch {
			case a.scope() == scopeBlock:
				to = "the section"
			case a.scope() == scopeSection && a.sentence != "":
				to = "the sentence only"
			}
			help = "enter send · tab ask about " + to + " · esc back"
		}
	case askStream:
		help = "esc cancel"
	case askAnswer:
		help = "enter reply · n new · esc back"
	}
	if a.review {
		switch {
		case a.mode == askAnswer:
			help = "enter reply or try again · esc carry on reading"
		case a.mode == askInput && len(a.entry.Turns) > 0:
			help = "enter send · esc back"
		case a.sec >= 0:
			help = "enter send · tab questions instead · esc skip"
		default:
			help = "enter send · esc cancel"
		}
	}
	help += " · " + view.KeysHint
	if maxScroll > 0 {
		help += fmt.Sprintf(" · %d/%d", a.scroll+bodyH, total)
	}
	return append(append(top, body...), "", theme.Fg(theme.Overlay0).Render(help))
}

// imageLines names the images of a question, under it.
func imageLines(images []string) []string {
	var out []string
	for _, p := range images {
		out = append(out, theme.Fg(theme.Overlay2).Render("  ▣ "+filepath.Base(p)))
	}
	return out
}

func (m *Model) historyPane(cw, h int) []string {
	a := m.ask
	out := []string{theme.Fg(theme.Subtext1).Render(fmt.Sprintf("history · %d asked", len(a.hist))), ""}
	listH := max(h-4, 1)
	if len(a.hist) == 0 {
		out = append(out, theme.Fg(theme.Overlay2).Render("no questions asked about this file yet"))
	}
	first := max(a.sel-listH+1, 0)
	for i := first; i < len(a.hist) && i < first+listH; i++ {
		e := a.hist[i]
		about := e.Block
		switch {
		case e.Section:
			about = m.askTitle(e)
		case e.Selection != "":
			about = e.Selection
		}
		block := []rune(strings.Join(strings.Fields(about), " "))
		if len(block) > 24 {
			block = append(block[:23], '…')
		}
		when := e.Time.Local().Format("Jan 2 15:04")
		q := ""
		if len(e.Turns) > 0 {
			q = e.Turns[0].Question
		}
		if len(e.Turns) > 1 {
			q += fmt.Sprintf(" (+%d)", len(e.Turns)-1)
		}
		line := []rune(fmt.Sprintf("%s  %s — %s", when, string(block), q))
		if len(line) > cw-2 {
			line = append(line[:max(cw-3, 1)], '…')
		}
		if i == a.sel {
			out = append(out, theme.Fg(theme.Yellow).Render("› ")+
				lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Foreground)).Bold(true).Render(string(line)))
		} else {
			out = append(out, "  "+theme.Fg(theme.Subtext0).Render(string(line)))
		}
	}
	for len(out) < h-1 {
		out = append(out, "")
	}
	out = out[:h-1]
	return append(out, theme.Fg(theme.Overlay0).Render("enter open · g go to block · esc back · "+view.KeysHint))
}
