package hold

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Look is how Underline draws: the colors of the filled part and of the
// track still to fill, as ANSI palette numbers (0–255), so they follow the
// terminal's theme. A negative Track draws no track.
type Look struct {
	Fill  int
	Track int
}

// DefaultLook fills in yellow over a dotted track in bright black.
var DefaultLook = Look{Fill: 3, Track: 8}

// sgr matches the style sequences inside already styled text, after which
// the underline is set again: a reset in the text would end it.
var sgr = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// Underline draws progress p (0 to 1) as an underline under columns from to
// width of an already styled line, padded with spaces to width. Anything
// running shows at least one column, so a hold shows the moment it starts.
// Terminals without colored underlines draw it in the text's color; those
// without styled ones draw the track as a plain underline.
func Underline(line string, from, width int, p float64, look Look) string {
	if w := ansi.StringWidth(line); w < width {
		line += strings.Repeat(" ", width-w)
	}
	from = min(max(from, 0), width)
	k := from + int(math.Ceil(p*float64(width-from)))
	k = min(max(k, from+1), width)
	out := ansi.Cut(line, 0, from) +
		under(ansi.Cut(line, from, k), fmt.Sprintf("\x1b[4m\x1b[58;5;%dm", look.Fill))
	if look.Track >= 0 {
		out += under(ansi.Cut(line, k, width), fmt.Sprintf("\x1b[4:4m\x1b[58;5;%dm", look.Track))
	} else {
		out += ansi.Cut(line, k, width)
	}
	return out
}

// under sets the underline on around s, and again after each style in it.
func under(s, on string) string {
	if s == "" {
		return ""
	}
	return on + sgr.ReplaceAllString(s, "${0}"+on) + "\x1b[59m\x1b[24m"
}
