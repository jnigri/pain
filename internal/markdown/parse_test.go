package markdown

import "testing"

func str(cs []Cell) string {
	rs := make([]rune, len(cs))
	for i, c := range cs {
		rs[i] = c.R
	}
	return string(rs)
}

func TestParseBlocks(t *testing.T) {
	src := "# 1. Intro\n\nHello   *world*.\n\n- one\n- two\n\n```go\nx := 1\n```\n"
	units := Parse([]byte(src))

	var got []string
	for _, u := range units {
		if u.Kind != Spacer {
			got = append(got, str(u.Cells))
		}
	}
	want := []string{"1. Intro", "Hello world.", "one", "two", "go", "x := 1"}
	if len(got) != len(want) {
		t.Fatalf("units = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unit %d = %q, want %q", i, got[i], want[i])
		}
	}
	if units[len(units)-1].Kind == Spacer {
		t.Error("trailing spacer not trimmed")
	}
}

func TestSkipNumbering(t *testing.T) {
	cs := skipNumbering(Cells("2.3 Title", 0, false))
	for i, c := range cs {
		if want := i < 4; c.Skip != want {
			t.Errorf("cell %d (%q) skip = %v, want %v", i, c.R, c.Skip, want)
		}
	}
	if cs := skipNumbering(Cells("2024 was", 0, false)); !cs[0].Skip {
		// A leading number followed by a space is treated as numbering.
		t.Error("expected leading number to be skipped")
	}
	if cs := skipNumbering(Cells("v2 notes", 0, false)); cs[0].Skip {
		t.Error("non-numeric start must not be skipped")
	}
}

func TestTidy(t *testing.T) {
	if got := str(tidy(Cells("  a   b  ", 0, false))); got != "a b" {
		t.Errorf("tidy = %q, want %q", got, "a b")
	}
	code := Cells("a  b", InlineCode, false)
	if got := str(tidy(code)); got != "a  b" {
		t.Errorf("tidy kept code spaces = %q, want %q", got, "a  b")
	}
}

func TestMatches(t *testing.T) {
	tests := []struct {
		typed, want rune
		cs          bool
		ok          bool
	}{
		{'a', 'a', false, true},
		{'A', 'a', false, true},
		{'A', 'a', true, false},
		{'\'', '’', false, true},
		{'"', '“', false, true},
		{'-', '—', false, true},
		{'e', 'é', false, true},
		{'x', 'y', false, false},
	}
	for _, tt := range tests {
		if got := Matches(tt.typed, tt.want, tt.cs); got != tt.ok {
			t.Errorf("matches(%q, %q, %v) = %v, want %v", tt.typed, tt.want, tt.cs, got, tt.ok)
		}
	}
}

func TestUntypeable(t *testing.T) {
	for _, r := range []rune{'→', '│', '😀'} {
		if !untypeable(r) {
			t.Errorf("untypeable(%q) = false", r)
		}
	}
	for _, r := range []rune{'a', 'é', '1', '.', 'ж'} {
		if untypeable(r) {
			t.Errorf("untypeable(%q) = true", r)
		}
	}
}
