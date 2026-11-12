package picture

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestFit(t *testing.T) {
	// A wide image takes the full width, a tall one the full height.
	if c, r := fitBlocks(800, 400, 80); c != 80 || r != 20 {
		t.Errorf("fitBlocks wide = %dx%d, want 80x20", c, r)
	}
	if c, r := fitBlocks(100, 400, 80); c != 10 || r != maxRows {
		t.Errorf("fitBlocks tall = %dx%d, want 10x%d", c, r, maxRows)
	}
	if c, r := fitBlocks(4, 4, 80); c != 4 || r != 2 {
		t.Errorf("fitBlocks small = %dx%d, want 4x2", c, r)
	}
	// 10 by 20 pixel cells: 200 by 100 pixels is 20 cells by 5.
	if c, r := fitKitty(200, 100, 80, 10, 20); c != 20 || r != 5 {
		t.Errorf("fitKitty small = %dx%d, want 20x5", c, r)
	}
	if c, r := fitKitty(4000, 1000, 80, 10, 20); c != 80 || r != 10 {
		t.Errorf("fitKitty wide = %dx%d, want 80x10", c, r)
	}
	if c, r := fitKitty(1000, 4000, 80, 10, 20); c != 10 || r != maxRows {
		t.Errorf("fitKitty tall = %dx%d, want 10x%d", c, r, maxRows)
	}
}

func TestBlockLines(t *testing.T) {
	// Red over blue on the left, see-through on the right.
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	img.Set(0, 1, color.NRGBA{0, 0, 255, 255})
	lines := blockLines(img, 2, 1, termenv.TrueColor)
	want := "\x1b[0;38;2;255;0;0;48;2;0;0;255m▀\x1b[0m \x1b[0m"
	if len(lines) != 1 || lines[0] != want {
		t.Errorf("blockLines = %q, want %q", lines, want)
	}
}

func TestPlaceholders(t *testing.T) {
	lines := placeholders(0x010203, 3, 2)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.HasPrefix(lines[0], "\x1b[38;2;1;2;3m") {
		t.Errorf("line does not start with the id as a color: %q", lines[0])
	}
	// Bubble Tea cuts lines by their width: a cell must count as one column.
	for _, l := range lines {
		if w := lipgloss.Width(l); w != 3 {
			t.Errorf("width = %d, want 3", w)
		}
	}
	if rs := []rune(strings.TrimPrefix(lines[1], "\x1b[38;2;1;2;3m")); rs[1] != diacritics[1] || rs[2] != diacritics[0] {
		t.Errorf("first cell of row 1 is not numbered row 1, column 0: %q", rs[:3])
	}
}

func TestTransmit(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 4000) // more than one chunk once in base64
	out := string(transmit(7, 4, 2, data, false))
	if !strings.HasPrefix(out, "\x1b_Ga=T,U=1,q=2,C=1,f=100,i=7,c=4,r=2,m=1;") {
		t.Errorf("bad first chunk: %q", out[:60])
	}
	if n := strings.Count(out, "\x1b_G"); n != 2 {
		t.Errorf("%d chunks, want 2", n)
	}
	if !strings.Contains(out, "\x1b\\\x1b_Gm=0;") {
		t.Error("the last chunk is not marked m=0")
	}
	in := string(transmit(7, 4, 2, data, true))
	if !strings.HasPrefix(in, "\x1bPtmux;\x1b\x1b_Ga=T") || !strings.HasSuffix(in, "\x1b\x1b\\\x1b\\") {
		t.Error("not wrapped for tmux")
	}
}
