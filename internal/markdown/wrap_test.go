package markdown

import "testing"

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
