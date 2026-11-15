package reader

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"pain/internal/markdown"
	"pain/internal/store"
)

func TestReadingNavigation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := []byte("First sentence. Second sentence.\n\nThird sentence. Fourth sentence.")
	for _, selection := range []scope{scopeSentence, scopeBlock, scopeSection} {
		m := New("/navigation.md", src, markdown.Parse(src), 0, store.Prefs{})
		m.sel = selection
		press := func(k tea.KeyType) { m.Update(tea.KeyMsg{Type: k}) }
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("First")})
		if len(m.typed) != 0 || m.words != 0 || !m.lastKey.IsZero() {
			t.Fatal("reading mode accepted typing")
		}
		press(tea.KeyEnter)
		if m.cur != 0 || string(m.typed) != "First sentence. " {
			t.Fatalf("enter skipped more than a sentence: %d %q", m.cur, m.typed)
		}
		press(tea.KeyCtrlW)
		if string(m.typed) != "First sentence. " {
			t.Fatal("reading mode deleted a word")
		}
		press(tea.KeySpace)
		if m.cur != m.firstTyping(1) {
			t.Fatal("space did not cross the block boundary")
		}
		press(tea.KeyDelete)
		if m.cur != 0 || string(m.typed) != "First sentence. " {
			t.Fatalf("delete did not return to previous sentence: %d %q", m.cur, m.typed)
		}
		press(tea.KeyBackspace)
		press(tea.KeyBackspace)
		if m.cur != 0 || len(m.typed) != 0 {
			t.Fatal("backspace did not stop at the first sentence")
		}
	}
}

func TestTypingNavigationUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := []byte("First sentence. Second sentence.")
	m := New("/typing.md", src, markdown.Parse(src), 0, store.Prefs{Typing: true})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("First")})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if string(m.typed) != "First " {
		t.Fatalf("enter: %q", m.typed)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if string(m.typed) != "First" {
		t.Fatalf("backspace: %q", m.typed)
	}
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if string(m.typed) != "First " {
		t.Fatalf("space: %q", m.typed)
	}
}
