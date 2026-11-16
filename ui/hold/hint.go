package hold

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Hint sets hint, already styled, at the right end of a styled line of
// width columns, such as "hold to answer" on the choice selected. The
// spaces padding the line make room for it; when its text leaves less than
// two columns free, the line stays as it is.
func Hint(line string, width int, hint string) string {
	used := ansi.StringWidth(strings.TrimRight(ansi.Strip(line), " "))
	hw := ansi.StringWidth(hint)
	if used+2+hw > width {
		return line
	}
	left := ansi.Truncate(line, width-hw, "")
	return left + strings.Repeat(" ", max(width-hw-ansi.StringWidth(left), 0)) + hint
}
