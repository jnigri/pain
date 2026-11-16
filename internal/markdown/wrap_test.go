package markdown

import (
	"strings"
	"testing"
)

func lines(cs []Cell, rs [][2]int) []string {
	var out []string
	for _, r := range rs {
		out = append(out, str(cs[r[0]:r[1]]))
	}
	return out
}

func TestWrapWords(t *testing.T) {
	cs := Cells("the quick brown fox jumps", 0, false)
	got := lines(cs, Wrap(cs, 10, false))
	want := []string{"the quick ", "brown fox ", "jumps"}
	if len(got) != len(want) {
		t.Fatalf("wrap = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWrapLongWord(t *testing.T) {
	cs := Cells("abcdefghijklmnopqrst", 0, false)
	for _, r := range Wrap(cs, 8, false) {
		if w := Width(cs[r[0]:r[1]]); w > 8 {
			t.Errorf("line %q is %d wide, want <= 8", str(cs[r[0]:r[1]]), w)
		}
	}
}

func TestWrapHard(t *testing.T) {
	cs := Cells("aaaa bbbb cc", 0, false)
	got := lines(cs, Wrap(cs, 8, true))
	if len(got) != 2 || got[0] != "aaaa bbb" || got[1] != "b cc" {
		t.Errorf("hard wrap = %q", got)
	}
}

func TestSentence(t *testing.T) {
	cs := Cells("One. Two three! Four", 0, false)
	tests := []struct {
		pos  int
		want string
	}{
		{0, "One."},
		{6, "Two three!"},
		{18, "Four"},
	}
	for _, tt := range tests {
		s, e := Sentence(cs, tt.pos)
		if got := str(cs[s:e]); got != tt.want {
			t.Errorf("sentence(pos=%d) = %q, want %q", tt.pos, got, tt.want)
		}
	}
}

// sentences splits s into the sentences Sentence finds, one after another.
func sentences(cs []Cell) []string {
	var out []string
	for pos := 0; pos < len(cs); {
		s, e := Sentence(cs, pos)
		out = append(out, str(cs[s:e]))
		pos = e
		for pos < len(cs) && cs[pos].R == ' ' {
			pos++
		}
	}
	return out
}

func TestSentenceSplits(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"Use a tool, e.g. Vim. Then save.", []string{"Use a tool, e.g. Vim.", "Then save."}},
		{"Some (i.e. most) are. Others not.", []string{"Some (i.e. most) are.", "Others not."}},
		{"Python vs. Go. Pick one.", []string{"Python vs. Go.", "Pick one."}},
		{"Ask Dr. Smith. She knows.", []string{"Ask Dr. Smith.", "She knows."}},
		{"See Fig. 3 and p. 12. Done.", []string{"See Fig. 3 and p. 12.", "Done."}},
		{"Say no. Then go.", []string{"Say no.", "Then go."}},
		{"It took approx. ten days, etc. and more.", []string{"It took approx. ten days, etc. and more."}},
		{"Apples, pears, etc. Then pie.", []string{"Apples, pears, etc.", "Then pie."}},
		{"Wait... then go. Or not!", []string{"Wait... then go.", "Or not!"}},
		{`He said "Stop." Then left.`, []string{`He said "Stop."`, "Then left."}},
		{"It works (mostly.) Try it?", []string{"It works (mostly.)", "Try it?"}},
		{"Well… Fine.", []string{"Well…", "Fine."}},
		{"Done. Then `go build`. ok", []string{"Done.", "Then `go build`. ok"}},
	}
	for _, tt := range tests {
		got := sentences(Cells(tt.in, 0, false))
		if strings.Join(got, "|") != strings.Join(tt.want, "|") {
			t.Errorf("%q split into %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSentenceInlineCode(t *testing.T) {
	cs := append(Cells("Run ", 0, false), Cells("a. B", InlineCode, false)...)
	cs = append(cs, Cells(" now. Next.", 0, false)...)
	if got := sentences(cs); len(got) != 2 || got[0] != "Run a. B now." {
		t.Errorf("split into %q", got)
	}
}
