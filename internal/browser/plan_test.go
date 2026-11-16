package browser

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pain/internal/store"
)

func planFolder(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	for name, s := range map[string]string{
		"a.md":                 "# A\n\nSome words in a.\n",
		"b.md":                 "# B\n\n## One\n\nFirst part words.\n\n## Two\n\nSecond part words.\n",
		"code.yaml":            "kind: Application\nspec: {}\nmore: 1\n",
		".reading-plans/p.md":  "# Plan\n\nWhy.\n\n- Start\n  - [B two](b.md#two)\n  - [Code](code.yaml#L1-L2)\n- [A](a.md) — note\n- [Gone](gone.md)\n- [Bad](b.md#nope)\n",
		".reading-plans/x.txt": "not a plan",
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPlanLoad(t *testing.T) {
	root, err := Load(planFolder(t))
	if err != nil {
		t.Fatal(err)
	}
	b := New(root, "")
	if len(root.kids) != 3 || !root.kids[0].plan || root.kids[1].name != "a.md" {
		t.Fatalf("kids %v", root.kids)
	}
	pl := root.kids[0]
	if pl.name != "Plan" || len(pl.kids) != 6 || pl.files != 3 {
		t.Fatalf("plan %q: %d rows, %d to read", pl.name, len(pl.kids), pl.files)
	}
	// The plan points at the folder's files: they count once.
	if root.files != 2 {
		t.Errorf("root counts %d files", root.files)
	}
	want := []struct {
		num            string
		group, missing bool
		frag           string
	}{
		{"1", true, false, ""},
		{"1.1", false, false, "two"},
		{"1.2", false, false, "L1-L2"},
		{"2", false, false, ""},
		{"3", false, true, ""},
		{"4", false, true, "nope"},
	}
	for i, w := range want {
		k := pl.kids[i]
		if k.num != w.num || k.group != w.group || k.missing != w.missing || k.Frag != w.frag {
			t.Errorf("row %d: %+v", i, k)
		}
	}
	if pl.kids[3].note != "note" || pl.kids[2].stats.units() != 3 {
		t.Errorf("note %q, code units %d", pl.kids[3].note, pl.kids[2].stats.units())
	}
	if pl.kids[0].files != 2 {
		t.Errorf("the group counts %d rows", pl.kids[0].files)
	}
	if b.View() != "" {
		t.Error("drawn before the size is known")
	}
	b.W, b.H = 100, 30
	if b.View() == "" {
		t.Error("nothing drawn")
	}
}

func TestPlanNavigate(t *testing.T) {
	dir := planFolder(t)
	root, _ := Load(dir)
	b := New(root, "")
	b.W, b.H = 100, 30
	b.Update(key("g"))
	if open, _ := b.Update(key("l")); open != nil || !b.dir.plan {
		t.Fatalf("l on the plan: opened %v, in %q", open, b.dir.name)
	}
	_ = b.View()
	// The group and the missing rows do not open.
	if open, _ := b.Update(key("l")); open != nil || !b.dir.plan {
		t.Fatal("a group opened")
	}
	b.Update(key("j"))
	open, _ := b.Update(key("enter"))
	if open == nil || open.Key != filepath.Join(dir, "b.md")+"#two" {
		t.Fatalf("opened %+v", open)
	}
	if b.Root.Find(open.Key) != open {
		t.Error("the row is not found by its key")
	}

	// Reading the part leaves the whole file unread.
	store.MarkDone(open.Key, open.stats.units())
	b.Refresh()
	whole := root.Find(filepath.Join(dir, "b.md"))
	if open.done != 1 || whole.done != 0 || whole.plan {
		t.Errorf("part done %d, whole file done %d", open.done, whole.done)
	}
	// The next row skips the group and what is missing.
	if u := b.NextUnread(open); u == nil || u.num != "1.2" {
		t.Errorf("after 1.1: %+v", u)
	}
	store.MarkDone(root.kids[0].kids[2].Key, 3)
	b.Refresh()
	if u := b.NextUnread(root.kids[0].kids[2]); u == nil || u.num != "2" {
		t.Errorf("after 1.2: %+v", u)
	}
	if g := root.kids[0].kids[0]; g.done != 2 || g.files != 2 {
		t.Errorf("group %d/%d", g.done, g.files)
	}
	b.Update(key("j"))
	b.Update(key("j"))
	b.Update(key("j"))
	if s := b.selected(); !s.missing {
		t.Fatalf("selected %+v", s)
	}
	if open, _ := b.Update(key("l")); open != nil {
		t.Error("a missing row opened")
	}
	b.Update(key("h"))
	if b.dir != root || b.selected() != root.kids[0] {
		t.Error("h does not go back to the plan")
	}
}
