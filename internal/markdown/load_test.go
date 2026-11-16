package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Applications":                      "applications",
		"App Of Apps Pattern (Alternative)": "app-of-apps-pattern-alternative",
		"Pass key-value pairs via values":   "pass-key-value-pairs-via-values",
		"1. Create the webhook":             "1-create-the-webhook",
		"Git Generator: Directories":        "git-generator-directories",
		"A + B":                             "a--b",
	} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func headings(units []Unit) []string {
	var out []string
	for _, u := range units {
		if u.Kind == Heading {
			out = append(out, str(u.Cells))
		}
	}
	return out
}

func TestSection(t *testing.T) {
	units := Parse([]byte("# Top\n\nintro\n\n## A\n\none\n\n### A sub\n\ntwo\n\n## B\n\nthree\n\n## A\n\nfour\n\n## Gen: X + Y\n\nfive\n"))
	for _, c := range []struct {
		anchor string
		want   []string
		last   string
	}{
		{"a", []string{"A", "A sub"}, "two"},
		{"a-sub", []string{"A sub"}, "two"},
		{"a-1", []string{"A"}, "four"},
		{"top", []string{"Top", "A", "A sub", "B", "A", "Gen: X + Y"}, "five"},
		{"gen-x--y", []string{"Gen: X + Y"}, "five"},
		{"gen-x-y", []string{"Gen: X + Y"}, "five"},
	} {
		from, to, ok := Section(units, c.anchor)
		if !ok {
			t.Errorf("%q: not found", c.anchor)
			continue
		}
		got := units[from:to]
		if h := headings(got); strings.Join(h, "|") != strings.Join(c.want, "|") {
			t.Errorf("%q: headings %q, want %q", c.anchor, h, c.want)
		}
		if last := str(got[len(got)-1].Cells); last != c.last {
			t.Errorf("%q: ends on %q, want %q", c.anchor, last, c.last)
		}
	}
	if _, _, ok := Section(units, "nope"); ok {
		t.Error("an unknown anchor was found")
	}
}

func TestLineRange(t *testing.T) {
	for _, c := range []struct {
		in       string
		from, to int
		ok       bool
	}{
		{"L10-L40", 10, 40, true},
		{"L7", 7, 7, true},
		{"l3-l4", 3, 4, true},
		{"L40-L10", 0, 0, false},
		{"L0", 0, 0, false},
		{"applications", 0, 0, false},
		{"", 0, 0, false},
	} {
		from, to, ok := LineRange(c.in)
		if from != c.from || to != c.to || ok != c.ok {
			t.Errorf("LineRange(%q) = %d, %d, %v", c.in, from, to, ok)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	yaml := write("app.yaml", "kind: Application\nspec:\n  # ```\n  project: default\n")
	units, _, err := Load(yaml)
	if err != nil {
		t.Fatal(err)
	}
	var code []string
	for _, u := range units {
		if u.Kind != CodeLine {
			t.Fatalf("unit %q is not code", str(u.Cells))
		}
		code = append(code, str(u.Cells))
	}
	if want := "yaml|kind: Application|spec:|  # ```|  project: default"; strings.Join(code, "|") != want {
		t.Errorf("code %q", code)
	}
	from, to, ok := Part(units, "L2-L3")
	if !ok || from != 2 || to != 4 || str(units[from].Cells) != "spec:" {
		t.Errorf("range: %v %d-%d", ok, from, to)
	}
	if from, to, ok := Part(units, "L1"); !ok || from != 1 || to != 2 {
		t.Errorf("one line: %v %d-%d", ok, from, to)
	}
	if _, _, ok := Part(units, "L9"); ok {
		t.Error("a range past the end was found")
	}

	md := write("doc.md", "# Doc\n\n## One\n\nfirst\n\n## Two\n\nsecond\nstill second\n\n- third\n")
	units, _, _ = Load(md)
	from, to, ok = Part(units, "two")
	if !ok || strings.Join(headings(units[from:to]), "|") != "Two" {
		t.Errorf("section: %v %q", ok, headings(units[from:to]))
	}
	if _, _, ok := Part(units, "three"); ok {
		t.Error("an unknown anchor was found")
	}
	// Lines pick the blocks that start in them, whole.
	from, to, ok = Part(units, "L9-L11")
	var got []string
	for _, u := range units[from:to] {
		got = append(got, str(u.Cells))
	}
	if !ok || strings.Join(got, "|") != "second still second" {
		t.Errorf("lines: %v %q", ok, got)
	}
	if from, to, _ := Part(units, ""); from != 0 || to != len(units) {
		t.Error("no fragment is not the whole file")
	}
	if _, _, err := Load(filepath.Join(dir, "none.md")); err == nil {
		t.Error("a missing file loaded")
	}
}
