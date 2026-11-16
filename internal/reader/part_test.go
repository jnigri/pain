package reader

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pain/internal/markdown"
	"pain/internal/store"
)

// Reading part of a file saves what is read for the whole file, so the
// folder and the reading plans share it.
func TestPartSharesProgress(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := []byte("# Doc\n\nIntro words.\n\n## One\n\nFirst part. Two sentences.\n\n## Two\n\nSecond part.\n")
	all := markdown.Parse(src)
	from, to, ok := markdown.Part(all, "one")
	if !ok {
		t.Fatal("no section")
	}
	const file = "/doc.md"
	m := NewPart(file+"#one", file, src, all, from, to, 0, store.Prefs{})
	if m.FilePath() != file || len(m.units) != to-from {
		t.Fatalf("%q, %d units", m.FilePath(), len(m.units))
	}
	m.Update(tea.KeyMsg{Type: tea.KeySpace}) // past the heading
	p := store.ReadProgress()[file]
	if p.Total != len(all) || !p.Covered().Has(from, from+m.cur) || p.Covered().Has(0, 1) {
		t.Fatalf("after one block: %+v", p)
	}
	for i := 0; i < 10 && !m.Finished; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeySpace})
	}
	if !m.Finished {
		t.Fatal("the part did not finish")
	}
	if p := store.ReadProgress()[file]; !reflect.DeepEqual(p.Read, store.Ranges{{from, to}}) || p.Done || p.Unit != 0 {
		t.Errorf("after the part: %+v", p)
	}
	if _, ok := store.ReadProgress()[file+"#one"]; ok {
		t.Error("progress saved under the row's key")
	}
	if got := store.LoadPart(file, len(all), from, to); got != from {
		t.Errorf("a part read through starts again at %d", got)
	}
}
