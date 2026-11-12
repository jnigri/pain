// Package view draws what the screens share: a unit as styled, wrapped lines,
// and the key help box.
package view

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"pain/internal/markdown"
	"pain/internal/picture"
	"pain/internal/theme"
)

// runKey is what makes neighbouring cells look the same.
type runKey struct {
	f     markdown.Attr
	st    theme.State
	focus bool
}

// Cursor is the typing state of the current unit.
type Cursor struct {
	Typed         []rune
	CaseSensitive bool
	WholeBlock    bool // underline the whole block rather than the sentence
}

// textWidth is the room left for u's cells at content width w, after the
// quote bars, indent, prefix and, for code, the background padding.
func textWidth(u *markdown.Unit, w int) int {
	avail := w - 2*u.Quote - u.Indent - runewidth.StringWidth(u.Prefix)
	if u.Kind == markdown.CodeLine {
		avail -= 2
	}
	return avail
}

// LineRanges is how u wraps on screen at content width w.
func LineRanges(u *markdown.Unit, w int) [][2]int {
	return markdown.Wrap(u.Cells, textWidth(u, w), u.Kind == markdown.CodeLine)
}

// RenderUnit draws unit u at content width w. c is nil for units that are not
// current; done tells whether such a unit is behind the cursor.
func RenderUnit(u *markdown.Unit, w int, c *Cursor, done bool) (lines []string, cursorLine int) {
	current := c != nil
	var typed []rune
	var sStart, sEnd int
	if current {
		typed = c.Typed
		sStart, sEnd = markdown.Sentence(u.Cells, len(typed))
		if c.WholeBlock {
			sStart, sEnd = 0, len(u.Cells)
		}
	}
	pending := !current && !done
	tint := func(hex string) string {
		if pending && hex != "" {
			return theme.Dim(hex)
		}
		return hex
	}

	bar := ""
	if u.Quote > 0 {
		bar = theme.Fg(tint(theme.Purple)).Render(strings.Repeat("┃ ", u.Quote))
	}
	leadW := 2*u.Quote + u.Indent
	lead := bar + strings.Repeat(" ", u.Indent)

	switch u.Kind {
	case markdown.Spacer:
		return []string{strings.TrimRight(bar, " ")}, 0
	case markdown.Rule:
		return []string{lead + theme.Fg(tint(theme.Overlay0)).Render(strings.Repeat("─", max(w-leadW, 1)))}, 0
	}

	pw := runewidth.StringWidth(u.Prefix)
	avail := textWidth(u, w)
	if u.Kind == markdown.Image {
		pic, ok := picture.Lines(u.Image, avail)
		if !ok {
			// What could not be drawn is named instead.
			name := strings.TrimSpace(cellText(u.Cells))
			if name == "" {
				name = filepath.Base(u.Image)
			}
			pic = []string{theme.Fg(tint(theme.Overlay0)).Italic(true).Render(runewidth.Truncate("▨ "+name, max(avail, 1), "…"))}
		}
		// A picture sits in the middle of the room it has.
		pad := ""
		if ok && len(pic) > 0 {
			pad = strings.Repeat(" ", max((avail-lipgloss.Width(pic[0]))/2, 0))
		}
		for i, l := range pic {
			l = pad + l
			if i == 0 {
				l = theme.Fg(tint(theme.PrefixColor(u))).Render(u.Prefix) + l
			} else {
				l = strings.Repeat(" ", pw) + l
			}
			lines = append(lines, lead+l)
		}
		return lines, 0
	}
	ranges := markdown.Wrap(u.Cells, avail, u.Kind == markdown.CodeLine)

	stateAt := func(i int) theme.State {
		switch {
		case !current && done:
			return theme.Done
		case !current:
			return theme.Pending
		case i < len(typed):
			if u.Cells[i].Skip || markdown.Matches(typed[i], u.Cells[i].R, c.CaseSensitive) {
				return theme.Done
			}
			return theme.Wrong
		case i == len(typed):
			return theme.Cursor
		}
		return theme.Pending
	}

	pos := len(typed)
	for li, rg := range ranges {
		var sb strings.Builder
		sb.WriteString(lead)
		if li == 0 {
			sb.WriteString(theme.Fg(tint(theme.PrefixColor(u))).Render(u.Prefix))
		} else {
			sb.WriteString(strings.Repeat(" ", pw))
		}
		band := theme.CellStyle(u, 0, theme.Done, false)
		if u.Kind == markdown.CodeLine {
			sb.WriteString(band.Render(" "))
		}

		// Group cells with the same look into one styled run.
		var run []rune
		var runStyle runKey
		flush := func() {
			if len(run) > 0 {
				out := theme.CellStyle(u, runStyle.f, runStyle.st, runStyle.focus).Render(string(run))
				if runStyle.f&markdown.Link != 0 && !runStyle.focus {
					out = theme.Dotted(out)
				}
				sb.WriteString(out)
				run = run[:0]
			}
		}
		for i := rg[0]; i < rg[1]; i++ {
			cl := u.Cells[i]
			st := stateAt(i)
			r := cl.R
			if st == theme.Wrong && r == ' ' {
				r = '·'
			}
			k := runKey{f: cl.Attr, st: st, focus: current && i >= sStart && i < sEnd}
			if k != runStyle || st == theme.Wrong {
				flush()
				runStyle = k
			}
			run = append(run, r)
		}
		flush()

		last := li == len(ranges)-1
		lineW := markdown.Width(u.Cells[rg[0]:rg[1]])
		if u.Kind == markdown.CodeLine {
			sb.WriteString(band.Render(strings.Repeat(" ", max(avail-lineW, 0)+1)))
		}
		if current && (pos >= rg[0] && pos < rg[1] || last && pos >= rg[1]) {
			cursorLine = li
		}
		lines = append(lines, sb.String())
	}

	if u.Kind == markdown.Heading && u.Level == 1 {
		rule := strings.Repeat("━", min(markdown.Width(u.Cells), avail))
		lines = append(lines, lead+theme.Fg(tint(theme.Dim(theme.Yellow))).Render(rule))
	}
	return lines, cursorLine
}

func cellText(cs []markdown.Cell) string {
	rs := make([]rune, len(cs))
	for i, c := range cs {
		rs[i] = c.R
	}
	return string(rs)
}
