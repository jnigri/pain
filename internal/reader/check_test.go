package reader

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pain/internal/markdown"
	"pain/internal/store"
)

func TestSplitSections(t *testing.T) {
	words := func(n int) string { return strings.Repeat("word ", n) + "\n\n" }

	small := markdown.Parse([]byte("# Title\n\n" + words(50) + "## Two\n\n" + words(50)))
	if s := splitSections(small); len(s) != 1 || s[0].end != len(small) {
		t.Errorf("small file: %+v, want one section", s)
	}

	// The short intro joins the first section, and the short tail the last.
	big := markdown.Parse([]byte("# Title\n\n" + words(20) + "## One\n\n" + words(300) +
		"## Two\n\n" + words(300) + "## Three\n\n" + words(20)))
	s := splitSections(big)
	if len(s) != 2 {
		t.Fatalf("big file: %d sections, want 2: %+v", len(s), s)
	}
	if s[0].title != "Title · One" || s[1].title != "Two · Three" {
		t.Errorf("titles %q and %q", s[0].title, s[1].title)
	}
	if s[0].start != 0 || s[0].end != s[1].start || s[1].end != len(big) {
		t.Errorf("sections do not cover the file: %+v", s)
	}
	if big[s[1].start].Kind != markdown.Heading {
		t.Error("second section does not start at its heading")
	}
	if sectionOf(s, 0) != 0 || sectionOf(s, s[1].start) != 1 || sectionOf(s, len(big)) != 1 {
		t.Error("sectionOf")
	}
}

func TestParseQuiz(t *testing.T) {
	in := "```json\n" + `{"sections":[
		{"questions":[{"q":"Q1","choices":["a","b","c","d"],"answer":2,"why":"w"},
		              {"q":"bad","choices":["a"],"answer":0}]},
		{"questions":[]}]}` + "\n```"
	quiz, err := parseQuiz(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(quiz) != 3 || len(quiz[0]) != 1 || len(quiz[1]) != 0 || len(quiz[2]) != 0 {
		t.Fatalf("got %+v", quiz)
	}
	// Shuffling keeps the answer on the right choice.
	if q := quiz[0][0]; q.Choices[q.Answer] != "c" {
		t.Errorf("answer points to %q, want c", q.Choices[q.Answer])
	}
	if _, err := parseQuiz("sorry, no", 1); err == nil {
		t.Error("no error without JSON")
	}
	if _, err := parseQuiz(`{"sections":[{"questions":[]}]}`, 1); err == nil {
		t.Error("no error without questions")
	}
}

func TestScopes(t *testing.T) {
	// New saves the position, which must not land in the real cache.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	words := func(n int) string { return strings.Repeat("word ", n) + "\n\n" }
	src := []byte("# Title\n\nOne two. Three four.\n\n" + words(300) + "## Two\n\n" + words(300))
	m := New("/x.md", src, markdown.Parse(src), 0, store.Prefs{SkipPunct: true})
	if len(m.secs) != 2 {
		t.Fatalf("%d sections, want 2", len(m.secs))
	}

	// The heading is one sentence: tab goes straight to the section and back.
	m.widen(1)
	if m.sel != scopeSection {
		t.Errorf("tab on a one-sentence block selects %v, want the section", scopeNames[m.sel])
	}
	m.widen(1)
	if m.sel != scopeSentence {
		t.Errorf("tab on the section selects %v, want the sentence", scopeNames[m.sel])
	}

	// A block of two sentences goes through all three, both ways.
	m.next()
	for _, want := range []scope{scopeBlock, scopeSection, scopeSentence} {
		if m.widen(1); m.sel != want {
			t.Errorf("tab selects %v, want %v", scopeNames[m.sel], scopeNames[want])
		}
	}
	if m.widen(-1); m.sel != scopeSection {
		t.Errorf("shift+tab selects %v, want the section", scopeNames[m.sel])
	}

	if got := m.toCheckpoint(); got != 304 {
		t.Errorf("toCheckpoint = %d, want 304", got)
	}
	first := m.cur
	m.nextSection()
	if sectionOf(m.secs, m.cur) != 1 || m.units[m.cur].Kind != markdown.Heading {
		t.Errorf("nextSection stops at unit %d", m.cur)
	}
	m.prevSection()
	if m.cur != 0 {
		t.Errorf("prevSection from a section start goes to unit %d, want 0", m.cur)
	}
	m.cur = first + 2
	m.prevSection()
	if m.cur != 0 {
		t.Errorf("prevSection inside a section goes to unit %d, want its start", m.cur)
	}
	m.nextSection()
	m.nextSection()
	if !m.Finished {
		t.Error("nextSection on the last section does not finish the file")
	}

	// The checkpoint mark sits under the last block that shows something.
	if last := m.secs[0].last(m.units); m.units[last].Kind != markdown.Para || last >= m.secs[0].end {
		t.Errorf("last of the first section is unit %d", last)
	}
}

func TestWrapHints(t *testing.T) {
	hs := []string{"aaaa", "bbbb", "cccc", "dddd"}
	for w, want := range map[int]int{30: 1, 20: 2, 12: 2, 6: 4} {
		lines := wrapHints(w, slices.Clone(hs))
		if len(lines) != want {
			t.Errorf("wrapHints(%d) = %d lines, want %d: %q", w, len(lines), want, lines)
		}
		for _, l := range lines {
			if lipgloss.Width(l) > max(w, 4) {
				t.Errorf("wrapHints(%d): line %q is too wide", w, l)
			}
		}
	}
}

func TestWritten(t *testing.T) {
	c := &checks{ch: make(chan tea.Msg), regen: -1}
	if w := c.written(); len(w) != 0 {
		t.Errorf("nothing written yet: %v", w)
	}
	c.partial = `{"sections":[{"questions":[{"q":"a","choices":["x"]},{"q": "b"}]},{"questions":[{"q":"c"`
	if w := c.written(); !slices.Equal(w, []int{2, 1}) {
		t.Errorf("written = %v, want [2 1]", w)
	}
}
