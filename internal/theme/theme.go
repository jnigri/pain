// Package theme maps a cell's style and typing state to a Lip Gloss style,
// in the terminal's own colors.
//
// Text takes the terminal's default foreground and its ANSI palette, and
// what is dimmed is drawn faint, so pain follows whatever theme the terminal
// has, light or dark, and repaints with it when the terminal switches. Only
// the surfaces behind code and the selected row need a real color: they are
// mixed from the terminal's background and foreground, asked for at start.
package theme

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"pain/internal/markdown"
)

// Color is a foreground: an ANSI palette color or the terminal's own, drawn
// faint or not.
type Color struct {
	c     lipgloss.TerminalColor // nil for the terminal's foreground
	faint bool
}

func ansi(n string) Color { return Color{c: lipgloss.Color(n)} }

var (
	Foreground = Color{}
	Muted      = Color{faint: true}
	Red        = ansi("1")
	Green      = ansi("2")
	Yellow     = ansi("3")
	Blue       = ansi("4")
	Purple     = ansi("5")
	Teal       = ansi("6")
	Pink       = ansi("9")
	Plan       = ansi("14") // reading plans: apart from blue folders and purple selection
)

// On sets c as the foreground of s.
func (c Color) On(s lipgloss.Style) lipgloss.Style {
	if c.c == nil {
		s = s.UnsetForeground()
	} else {
		s = s.Foreground(c.c)
	}
	return s.Faint(c.faint)
}

func Fg(c Color) lipgloss.Style { return c.On(lipgloss.NewStyle()) }

// Dim is c faint, for text not typed yet.
func Dim(c Color) Color { c.faint = true; return c }

// The surfaces for a dark and a light background. Those of the background
// pain started on are mixed from the terminal's colors; the other ones are
// a neutral guess, used if the terminal switches.
var (
	dark     = true
	surfaces = map[bool][2]string{
		true:  {"#2c2d2f", "#56585b"},
		false: {"#dfe1e3", "#a9adb1"},
	}
)

// The text being read when not typing, a step brighter than the terminal's
// foreground: mixed from it at start, a palette guess otherwise.
var brights = map[bool]Color{true: ansi("15"), false: ansi("0")}

// Bright is the foreground pushed further from the background, for the
// sentence being read.
func Bright() Color { return brights[dark] }

// Surface is the background behind code and the selected row.
func Surface() lipgloss.TerminalColor { return lipgloss.Color(surfaces[dark][0]) }

// Edge is the color of box borders.
func Edge() lipgloss.TerminalColor { return lipgloss.Color(surfaces[dark][1]) }

// Detect asks the terminal for its colors, to tell whether its background
// is dark and to mix the surfaces from them. A terminal that does not answer
// is taken to follow the system's appearance. Call it before the program
// starts reading the terminal.
func Detect() {
	out := termenv.NewOutput(os.Stdout)
	bg, ok1 := out.BackgroundColor().(termenv.RGBColor)
	fg, ok2 := out.ForegroundColor().(termenv.RGBColor)
	if !ok1 || !ok2 {
		dark = systemDark()
		return
	}
	b, f := string(bg), string(fg)
	dark = luminance(b) < 0.5
	surfaces[dark] = [2]string{blend(b, f, 0.1), blend(b, f, 0.35)}
	far := "#000000"
	if dark {
		far = "#ffffff"
	}
	brights[dark] = Color{c: lipgloss.Color(blend(f, far, 0.6))}
}

// systemDark tells whether macOS is in dark mode; elsewhere it guesses dark.
func systemDark() bool {
	if runtime.GOOS != "darwin" {
		return true
	}
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	return err != nil || strings.TrimSpace(string(out)) == "Dark"
}

// ChangedMsg tells the models the terminal switched between dark and light,
// so what they keep drawn is stale.
type ChangedMsg struct{}

// Watch asks the terminal to say when it switches between dark and light
// (mode 2031, in Ghostty, kitty, tmux and others).
func Watch() { os.Stdout.WriteString("\x1b[?2031h") }

// Unwatch stops the reports Watch asked for.
func Unwatch() { os.Stdout.WriteString("\x1b[?2031l") }

// Filter is a tea.WithFilter that turns the terminal's dark/light report
// (CSI ? 997 ; 1 n for dark, 2 for light) into a ChangedMsg. Bubble Tea
// hands it over as an unknown sequence, a byte slice of an unexported type.
func Filter(_ tea.Model, msg tea.Msg) tea.Msg {
	v := reflect.ValueOf(msg)
	if !v.IsValid() || v.Kind() != reflect.Slice || v.Type().Elem().Kind() != reflect.Uint8 {
		return msg
	}
	switch string(v.Bytes()) {
	case "\x1b[?997;1n":
		setDark(true)
	case "\x1b[?997;2n":
		setDark(false)
	default:
		return msg
	}
	return ChangedMsg{}
}

func setDark(d bool) {
	dark = d
	styleCache = map[styleKey]lipgloss.Style{}
}

type State int

const (
	Done State = iota
	Pending
	Wrong
	Cursor
	Reading // the sentence being read, when not typing
)

type styleKey struct {
	kind  markdown.Kind
	level int
	quote bool
	f     markdown.Attr
	st    State
}

var styleCache = map[styleKey]lipgloss.Style{}

func headingColor(level int) Color {
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
func PrefixColor(u *markdown.Unit) Color {
	if u.Ordered {
		return Yellow
	}
	return []Color{Pink, Purple, Blue}[u.ListDepth%3]
}

// Cell draws text with the look of a cell. The underline of the sentence
// being typed and the strikethrough are set around it rather than by Lip
// Gloss, which draws the spaces under them without the faint attribute: the
// underline of a dimmed sentence would light up between the words.
func Cell(u *markdown.Unit, f markdown.Attr, st State, focus bool, text string) string {
	out := CellStyle(u, f, st).Render(text)
	if f&markdown.Strike != 0 {
		out = "\x1b[9m" + out + "\x1b[29m"
	}
	if focus {
		out = "\x1b[4m" + out + "\x1b[24m"
	}
	return out
}

// CellStyle is the look of a cell, without its underline or strikethrough.
func CellStyle(u *markdown.Unit, f markdown.Attr, st State) lipgloss.Style {
	k := styleKey{u.Kind, u.Level, u.Quote > 0, f, st}
	if s, ok := styleCache[k]; ok {
		return s
	}
	fg := Foreground
	var bg lipgloss.TerminalColor
	var bold, ital, reverse bool
	switch u.Kind {
	case markdown.Heading:
		fg, bold = headingColor(u.Level), true
	case markdown.CodeLine:
		bg = Surface()
	}
	if u.Quote > 0 && u.Kind != markdown.CodeLine {
		fg, ital = Muted, true
	}
	if f&markdown.Bold != 0 {
		bold = true
	}
	if f&markdown.Italic != 0 {
		ital = true
	}
	if f&markdown.InlineCode != 0 {
		fg, bg = Green, Surface()
	}
	if f&markdown.Link != 0 {
		fg = Blue // underlined dotted by the renderer, see dotted
	}
	if f&markdown.Strike != 0 {
		fg = Muted
	}
	if f&markdown.TableHeader != 0 {
		fg, bold = Teal, true
	}
	if f&markdown.Mark != 0 {
		fg = Muted
	}
	switch st {
	case Pending:
		fg = Dim(fg)
	case Wrong:
		fg = Red
	case Cursor:
		// Yellow behind the letter, which takes the background's color.
		fg, reverse = Yellow, true
	case Reading:
		// Lit rather than dimmed: plain text a step brighter, colors as
		// they are.
		fg.faint = false
		if fg.c == nil {
			fg = Bright()
		}
	}
	s := fg.On(lipgloss.NewStyle()).Bold(bold).Italic(ital).Reverse(reverse)
	if bg != nil {
		s = s.Background(bg)
	}
	styleCache[k] = s
	return s
}

// Dotted wraps already styled text in a dotted underline (SGR 4:4), so links
// stay distinct from the solid underline of the focused sentence. Terminals
// without styled underlines drop it and links show by color alone.
func Dotted(s string) string { return "\x1b[4:4m" + s + "\x1b[24m" }

func luminance(h string) float64 {
	r, g, b := rgb(h)
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255
}

func blend(a, b string, t float64) string {
	ar, ag, ab := rgb(a)
	br, bg, bb := rgb(b)
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t + 0.5) }
	return fmt.Sprintf("#%02x%02x%02x", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func rgb(h string) (uint8, uint8, uint8) {
	v, _ := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 32)
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}
