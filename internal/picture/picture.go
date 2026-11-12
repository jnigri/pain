// Package picture draws an image file as lines of terminal text. Terminals
// that speak the Kitty graphics protocol get the real picture, sent once and
// then shown through placeholder characters; the others get it in half-block
// characters, two pixels to a cell.
package picture

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// maxRows caps how tall a picture is drawn, so one never fills the screen.
const maxRows = 20

// maxPixels is the largest image decoded.
const maxPixels = 64 << 20

type mode int

const (
	unset mode = iota
	off
	blocks
	kitty
)

type key struct {
	path string
	w    int
}

var (
	// Out is where pictures are sent to the terminal, ahead of the lines
	// that show them.
	Out io.Writer = os.Stdout

	how      mode
	inTmux   bool
	images   = map[string]image.Image{} // decoded files, nil for the unreadable
	drawn    = map[key][]string{}
	sent     []uint32 // image ids the terminal holds
	lastID   = rand.Uint32()
	pixelsOf = cellSize
)

// detect picks how pictures are drawn. PAIN_IMAGES forces it.
func detect() mode {
	inTmux = os.Getenv("TMUX") != ""
	switch os.Getenv("PAIN_IMAGES") {
	case "off":
		return off
	case "blocks":
		return blocks
	case "kitty":
		return kitty
	}
	if lipgloss.ColorProfile() == termenv.Ascii {
		return off
	}
	term := os.Getenv("TERM")
	graphics := os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("GHOSTTY_RESOURCES_DIR") != "" ||
		strings.Contains(term, "kitty") || strings.Contains(term, "ghostty")
	if graphics && (!inTmux || tmuxPassthrough()) {
		return kitty
	}
	return blocks
}

// tmuxPassthrough tells whether tmux lets the graphics escape codes through
// to the terminal around it.
func tmuxPassthrough() bool {
	out, err := exec.Command("tmux", "show-options", "-A", "-p", "-v", "allow-passthrough").Output()
	v := strings.TrimSpace(string(out))
	return err == nil && (v == "on" || v == "all")
}

// Lines draws the image at path at most w columns wide. ok is false when
// there is nothing to show: the file is missing or not an image, or the
// terminal has no colors.
func Lines(path string, w int) (lines []string, ok bool) {
	if how == unset {
		how = detect()
	}
	if how == off || w < 1 {
		return nil, false
	}
	img := load(path)
	if img == nil {
		return nil, false
	}
	k := key{path, w}
	if l, ok := drawn[k]; ok {
		return l, true
	}
	size := img.Bounds().Size()
	if how == kitty {
		cw, ch, known := pixelsOf()
		cols, rows := fitKitty(size.X, size.Y, w, cw, ch)
		// Without the real cell size, keep twice the pixels to stay sharp.
		pw, ph := cols*cw, rows*ch
		if !known {
			pw, ph = 2*pw, 2*ph
		}
		var data bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := enc.Encode(&data, shrink(img, pw, ph)); err != nil {
			return nil, false
		}
		lastID = lastID%0xffffff + 1 // 1 to 0xffffff: the id travels as a color
		// One Write, so the frames Bubble Tea writes cannot cut into it.
		if _, err := Out.Write(transmit(lastID, cols, rows, data.Bytes(), inTmux)); err != nil {
			return nil, false
		}
		sent = append(sent, lastID)
		lines = placeholders(lastID, cols, rows)
	} else {
		cols, rows := fitBlocks(size.X, size.Y, w)
		lines = blockLines(img, cols, rows, lipgloss.ColorProfile())
	}
	drawn[k] = lines
	return lines, true
}

// Release frees the pictures the terminal holds. Call it on the way out.
func Release() {
	var buf []byte
	for _, id := range sent {
		buf = append(buf, wrap(fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id), inTmux)...)
	}
	if len(buf) > 0 {
		_, _ = Out.Write(buf)
	}
	sent, drawn = nil, map[key][]string{}
}

func load(path string) image.Image {
	if img, ok := images[path]; ok {
		return img
	}
	images[path] = nil
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxPixels {
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	images[path] = img
	return img
}

// shrink scales img down to fit in w by h pixels. A smaller one is left as is.
func shrink(img image.Image, w, h int) image.Image {
	size := img.Bounds().Size()
	s := min(float64(w)/float64(size.X), float64(h)/float64(size.Y))
	if s >= 1 {
		return img
	}
	dst := image.NewNRGBA(image.Rect(0, 0, max(int(float64(size.X)*s), 1), max(int(float64(size.Y)*s), 1)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

// fitKitty is the box of cells for an image of w by h pixels, at most maxW
// columns wide, where a cell is cw by ch pixels. It never grows the image.
func fitKitty(w, h, maxW, cw, ch int) (cols, rows int) {
	s := min(1, float64(maxW*cw)/float64(w), float64(maxRows*ch)/float64(h))
	cols = int(math.Ceil(float64(w) * s / float64(cw)))
	rows = int(math.Ceil(float64(h) * s / float64(ch)))
	return min(max(cols, 1), maxW, len(diacritics)), min(max(rows, 1), maxRows)
}

// fitBlocks is the same for half blocks: a cell is one pixel wide and two tall.
func fitBlocks(w, h, maxW int) (cols, rows int) {
	cols = min(maxW, w)
	rows = (cols*h + 2*w - 1) / (2 * w)
	if rows > maxRows {
		rows = maxRows
		cols = max(rows*2*w/h, 1)
	}
	return cols, max(rows, 1)
}

// blockLines draws img in cols by rows cells. Each cell holds two pixels: the
// top one is the color of a ▀, the bottom one its background.
func blockLines(img image.Image, cols, rows int, p termenv.Profile) []string {
	px := image.NewNRGBA(image.Rect(0, 0, cols, 2*rows))
	draw.CatmullRom.Scale(px, px.Bounds(), img, img.Bounds(), draw.Src, nil)
	seq := func(x, y int, bg bool) string {
		c := px.NRGBAAt(x, y)
		if c.A < 128 {
			return "" // see-through: the terminal's own background
		}
		return p.Color(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)).Sequence(bg)
	}
	lines := make([]string, rows)
	for y := range rows {
		var sb strings.Builder
		last := ""
		for x := range cols {
			top, bottom := seq(x, 2*y, false), seq(x, 2*y+1, true)
			r := "▀"
			switch {
			case top == "" && bottom == "":
				r = " "
			case top == "":
				r, bottom = "▄", seq(x, 2*y+1, false)
			}
			style := strings.Trim(top+";"+bottom, ";")
			if style != last {
				sb.WriteString("\x1b[0")
				if style != "" {
					sb.WriteString(";" + style)
				}
				sb.WriteString("m")
				last = style
			}
			sb.WriteString(r)
		}
		sb.WriteString("\x1b[0m")
		lines[y] = sb.String()
	}
	return lines
}

// placeholder is the character a Kitty terminal draws an image over. The
// text color names the image, two diacritics the row and column of the cell.
const placeholder = '\U0010EEEE'

func placeholders(id uint32, cols, rows int) []string {
	lines := make([]string, rows)
	for r := range rows {
		var sb strings.Builder
		fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", id>>16&0xff, id>>8&0xff, id&0xff)
		for c := range cols {
			sb.WriteRune(placeholder)
			sb.WriteRune(diacritics[r])
			sb.WriteRune(diacritics[c])
		}
		sb.WriteString("\x1b[39m")
		lines[r] = sb.String()
	}
	return lines
}

// transmit is the escape codes that send a PNG to the terminal as image id,
// to be shown through placeholders in a box of cols by rows cells.
func transmit(id uint32, cols, rows int, data []byte, tmux bool) []byte {
	const chunk = 4096
	b64 := base64.StdEncoding.EncodeToString(data)
	var out []byte
	for first := true; first || len(b64) > 0; first = false {
		n := min(chunk, len(b64))
		more := 0
		if n < len(b64) {
			more = 1
		}
		ctl := fmt.Sprintf("m=%d", more)
		if first {
			// q=2: no reply, which would come back as typed keys.
			ctl = fmt.Sprintf("a=T,U=1,q=2,C=1,f=100,i=%d,c=%d,r=%d,m=%d", id, cols, rows, more)
		}
		out = append(out, wrap("\x1b_G"+ctl+";"+b64[:n]+"\x1b\\", tmux)...)
		b64 = b64[n:]
	}
	return out
}

// wrap passes an escape code through tmux to the terminal around it.
func wrap(s string, tmux bool) string {
	if !tmux {
		return s
	}
	return "\x1bPtmux;" + strings.ReplaceAll(s, "\x1b", "\x1b\x1b") + "\x1b\\"
}
