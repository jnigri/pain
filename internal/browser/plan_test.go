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
		".reading-plans/q.md":  "- [B one](b.md#one)\n- [B two again](b.md#two)\n",
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
	if len(root.kids) != 4 || !root.kids[0].plan || !root.kids[1].plan || root.kids[2].name != "a.md" {
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
	// The code row reads lines 1 and 2: the units after the language label.
	if code := pl.kids[2]; pl.kids[3].note != "note" || code.from != 1 || code.to != 3 || code.stats.units() != 4 {
		t.Errorf("note %q, code units %d-%d of %d", pl.kids[3].note, code.from, code.to, code.stats.units())
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

	// Reading a part counts for the file in the folder and in every plan.
	bmd := filepath.Join(dir, "b.md")
	total := open.stats.units()
	store.MarkRead(bmd, total, open.from, open.to)
	b.Refresh()
	whole := root.Find(bmd)
	again, one := root.kids[1].kids[1], root.kids[1].kids[0]
	if open.done != 1 || again.done != 1 || one.done != 0 || whole.plan || whole.done != 0 || whole.read != open.total {
		t.Errorf("part done %d, same part in another plan %d, other part %d, whole file %d/%d words",
			open.done, again.done, one.done, whole.read, whole.total)
	}
	// The next row skips the group and what is missing.
	if u := b.NextUnread(open); u == nil || u.num != "1.2" {
		t.Errorf("after 1.1: %+v", u)
	}
	code := root.kids[0].kids[2]
	store.MarkRead(code.Path, code.stats.units(), code.from, code.to)
	b.Refresh()
	if u := b.NextUnread(code); u == nil || u.num != "2" {
		t.Errorf("after 1.2: %+v", u)
	}
	if g := root.kids[0].kids[0]; g.done != 2 || g.files != 2 {
		t.Errorf("group %d/%d", g.done, g.files)
	}
	// Reading the whole file ticks its parts.
	store.SaveProgress(bmd, total, total)
	b.Refresh()
	if one.done != 1 || whole.done != 1 {
		t.Errorf("after the whole file: part %d, file %d", one.done, whole.done)
	}
	// x on a part marks only that part unread.
	b.Select(again)
	b.Update(key("x"))
	if again.done != 0 || open.done != 0 || one.done != 1 || whole.done != 0 {
		t.Errorf("after x: %d %d %d %d", again.done, open.done, one.done, whole.done)
	}
	b.Select(open)
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
