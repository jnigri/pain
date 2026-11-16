package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pain/internal/markdown"
)

func descText(units []markdown.Unit) string {
	var parts []string
	for _, u := range units {
		var sb strings.Builder
		for _, c := range u.Cells {
			sb.WriteRune(c.R)
		}
		if sb.Len() > 0 {
			parts = append(parts, sb.String())
		}
	}
	return strings.Join(parts, "|")
}

func TestParse(t *testing.T) {
	src := `# Argo CD: zero to ApplicationSet

Who it is for.

## Why

To **understand** apps:

- a list in the description

The plan:

- Foundations
  - [What Argo CD is](01-overview.md)
  - [Architecture](02-architecture.md) — look at the diagram
- [Application](06-declarative-setup.md#applications): the core
  - [Every field](files/application.yaml#L213-L258)
- [Elsewhere](/abs/doc.md)
- [Home](~/notes/a%20b.md)

Text after the list is ignored.
`
	pl := Parse([]byte(src), "/root")
	if pl.Title != "Argo CD: zero to ApplicationSet" {
		t.Errorf("title %q", pl.Title)
	}
	if got := descText(pl.Desc); got != "Who it is for.|Why|To understand apps:|a list in the description|The plan:" {
		t.Errorf("description %q", got)
	}
	home, _ := os.UserHomeDir()
	want := []Item{
		{Num: "1", Title: "Foundations"},
		{Num: "1.1", Title: "What Argo CD is", Path: "/root/01-overview.md", Depth: 1},
		{Num: "1.2", Title: "Architecture", Note: "look at the diagram", Path: "/root/02-architecture.md", Depth: 1},
		{Num: "2", Title: "Application", Note: "the core", Path: "/root/06-declarative-setup.md", Frag: "applications"},
		{Num: "2.1", Title: "Every field", Path: "/root/files/application.yaml", Frag: "L213-L258", Depth: 1},
		{Num: "3", Title: "Elsewhere", Path: "/abs/doc.md"},
		{Num: "4", Title: "Home", Path: filepath.Join(home, "notes/a b.md")},
	}
	if len(pl.Items) != len(want) {
		t.Fatalf("%d items, want %d: %+v", len(pl.Items), len(want), pl.Items)
	}
	for i, w := range want {
		if pl.Items[i] != w {
			t.Errorf("item %d = %+v, want %+v", i, pl.Items[i], w)
		}
	}
	if !pl.Items[0].Group() || pl.Items[1].Group() {
		t.Error("groups are the items without a link")
	}
}

func TestParseEdges(t *testing.T) {
	pl := Parse([]byte("Just prose, no title and no list.\n"), "/root")
	if pl.Title != "" || len(pl.Items) != 0 || descText(pl.Desc) != "Just prose, no title and no list." {
		t.Errorf("%+v", pl)
	}
	pl = Parse([]byte("- [A](a.md)\n"), "/root")
	if len(pl.Desc) != 0 || len(pl.Items) != 1 {
		t.Errorf("%+v", pl)
	}
	// A heading that is not first is part of the description.
	pl = Parse([]byte("Intro\n\n# Later\n\n1. [A](a.md)\n2. [B](b.md)\n"), "/root")
	if pl.Title != "" || descText(pl.Desc) != "Intro|Later" || len(pl.Items) != 2 || pl.Items[1].Num != "2" {
		t.Errorf("%+v", pl)
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	if got := Load(root); len(got) != 0 {
		t.Errorf("no folder: %d plans", len(got))
	}
	dir := filepath.Join(root, Dir)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{
		"b.md":      "# Bee\n\n- [x](x.md)\n",
		"a.md":      "- [y](y.md)\n",
		"notes.txt": "not a plan",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := Load(root)
	if len(got) != 2 || got[0].Title != "a" || got[1].Title != "Bee" {
		t.Fatalf("%+v", got)
	}
	if got[0].Items[0].Path != filepath.Join(root, "y.md") || got[1].File != filepath.Join(dir, "b.md") {
		t.Errorf("%+v", got)
	}
}

func TestCheck(t *testing.T) {
	root := t.TempDir()
	write := func(name, s string) {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var sb strings.Builder
	if Check(&sb, root) != 1 || !strings.Contains(sb.String(), "no plans") {
		t.Errorf("no plans: %q", sb.String())
	}
	write("doc.md", "# Doc\n\n## One\n\nThree words here.\n\n## One\n\nMore.\n")
	write("code.go", "package x\n")
	write(Dir+"/p.md", "# P\n\n- Group\n  - [one](doc.md#one)\n  - [code](code.go#L1)\n- [bad](doc.md#two)\n- [gone](gone.md)\n- [far](code.go#L5)\n")
	sb.Reset()
	n := Check(&sb, root)
	out := sb.String()
	for _, want := range []string{
		".reading-plans/p.md", "2 rows to read",
		"doc.md#one · 4 words",
		"MISSING doc.md#two: no heading with that anchor; the file has: doc, one, one-1",
		"MISSING gone.md: no such file or directory",
		"MISSING code.go#L5: no such lines",
		"3 problem(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if n != 3 {
		t.Errorf("%d problems", n)
	}
}
