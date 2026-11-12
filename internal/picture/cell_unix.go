//go:build unix

package picture

import (
	"os"

	"golang.org/x/sys/unix"
)

// cellSize is the size of a terminal cell in pixels. known is false when the
// terminal does not tell, and the size is then a guess.
func cellSize() (w, h int, known bool) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel < ws.Col || ws.Ypixel < ws.Row {
		return 10, 20, false
	}
	return int(ws.Xpixel / ws.Col), int(ws.Ypixel / ws.Row), true
}
