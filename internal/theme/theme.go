// Package theme holds the 0x96f palette and maps a cell's style and typing
// state to a Lip Gloss style.
package theme

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/lipgloss"

	"pain/internal/markdown"
)

// 0x96f palette, same values as the ghostty / tmux / nvim theme.
const (
	Background = "#262427"
	Foreground = "#fcfcfa"
	Subtext1   = "#c1c0c0"
	Subtext0   = "#a5a2a2"
	Overlay2   = "#727072"
	Overlay0   = "#545452"
	Surface0   = "#353236"
	Red        = "#ff666d"
	Pink       = "#ff7e83"
	Yellow     = "#ffc739"
	Green      = "#b3e03a"
	Teal       = "#9deaf6"
	Blue       = "#00cde8"
	Purple     = "#a392e8"
)

type State int

const (
	Done State = iota
	Pending
	Wrong
	Cursor
)

type styleKey struct {
	kind  markdown.Kind
	level int
	quote bool
	f     markdown.Attr
	st    State
	focus bool // in the sentence being typed
}

var styleCache = map[styleKey]lipgloss.Style{}

func headingColor(level int) string {
	switch level {
	case 1:
		return Yellow
	case 2:
		return Pink
	case 3:
		return Purple
	}
	return Teal
}

// PrefixColor is the color of a unit's list marker: numbers in yellow,
// bullets cycling through pink, purple and blue by nesting.
func PrefixColor(u *markdown.Unit) string {
	if u.Ordered {
		return Yellow
	}
	return []string{Pink, Purple, Blue}[u.ListDepth%3]
}

func CellStyle(u *markdown.Unit, f markdown.Attr, st State, focus bool) lipgloss.Style {
	k := styleKey{u.Kind, u.Level, u.Quote > 0, f, st, focus}
	if s, ok := styleCache[k]; ok {
		return s
	}
	fg, bg := Foreground, ""
	var bold, ital, under, strike bool
	switch u.Kind {
	case markdown.Heading:
		fg, bold = headingColor(u.Level), true
	case markdown.CodeLine:
		fg, bg = Subtext1, Surface0
	}
	if u.Quote > 0 && u.Kind != markdown.CodeLine {
		fg, ital = Subtext1, true
	}
	if f&markdown.Bold != 0 {
		bold = true
	}
	if f&markdown.Italic != 0 {
		ital = true
	}
	if f&markdown.InlineCode != 0 {
		fg, bg = Green, Surface0
	}
	if f&markdown.Link != 0 {
		fg = Blue // underlined dotted by the renderer, see dotted
	}
	if f&markdown.Strike != 0 {
		fg, strike = Subtext0, true
	}
	if f&markdown.TableHeader != 0 {
		fg, bold = Teal, true
	}
	if f&markdown.Mark != 0 {
		fg = Overlay2
	}
	switch st {
	case Pending:
		fg = Dim(fg)
	case Wrong:
		fg = Red
	case Cursor:
		fg, bg = Background, Yellow
	}
	if focus {
		under = true
	}
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).
		Bold(bold).Italic(ital).Underline(under).Strikethrough(strike)
	if bg != "" {
		s = s.Background(lipgloss.Color(bg))
	}
	styleCache[k] = s
	return s
}

// Dotted wraps already styled text in a dotted underline (SGR 4:4), so links
// stay distinct from the solid underline of the focused sentence. Terminals
// without styled underlines drop it and links show by color alone.
func Dotted(s string) string { return "\x1b[4:4m" + s + "\x1b[24m" }

func Fg(hex string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
}

// Dim pulls a color halfway toward the background, for text not typed yet.
func Dim(hex string) string { return blend(hex, Background, 0.5) }

func blend(a, b string, t float64) string {
	ar, ag, ab := rgb(a)
	br, bg, bb := rgb(b)
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t + 0.5) }
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func rgb(h string) (uint8, uint8, uint8) {
	v, _ := strconv.ParseUint(h[1:], 16, 32)
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}
