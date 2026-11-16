// Package plan reads reading plans: Markdown files in a folder's
// .reading-plans/ that list what to read, in which order, and why.
//
// A plan is a title (its first # heading), a description and a nested list:
// the last list of the file, so the description can hold lists too. A list item with a link is something to
// read: the link text is its title, the text after the link a note. An item
// without a link is a group. A link may end in a heading anchor
// (#applications) or a line range (#L10-L40) to read only part of a file.
package plan

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"pain/internal/markdown"
)

// Dir is the folder, inside the folder read, that holds its plans.
const Dir = ".reading-plans"

type Plan struct {
	Title string
	File  string
	Desc  []markdown.Unit
	Items []Item
}

// Item is one row of a plan's outline, in reading order.
type Item struct {
	Num   string // "2.1", from the place in the outline
	Title string
	Note  string
	Path  string // absolute, empty for a group
	Frag  string // the part of the file: a heading anchor or a line range
	Depth int    // 0 at the top of the outline
}

// Group tells whether the item only gathers the items under it.
func (it Item) Group() bool { return it.Path == "" }

// Load reads every plan in root's .reading-plans/, sorted by file name.
func Load(root string) []Plan {
	dir := filepath.Join(root, Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Plan
	for _, e := range entries {
		if e.IsDir() || !markdown.IsMarkdown(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		pl := Parse(src, root)
		pl.File = p
		if pl.Title == "" {
			pl.Title = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		}
		out = append(out, pl)
	}
	return out
}

// Parse reads one plan. Relative paths start from root.
func Parse(src []byte, root string) Plan {
	doc := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(src))
	var pl Plan
	descFrom, descTo := 0, len(src)
	if h, ok := doc.FirstChild().(*ast.Heading); ok && h.Level == 1 {
		pl.Title = strings.TrimSpace(plain(h, src))
		descFrom = lineEnd(src, h)
	}
	for n := doc.LastChild(); n != nil; n = n.PreviousSibling() {
		if l, ok := n.(*ast.List); ok {
			descTo = lineStart(src, l)
			pl.Items = items(l, src, root, "", 0, nil)
			break
		}
	}
	if descFrom < descTo {
		pl.Desc = markdown.Parse(src[descFrom:descTo])
	}
	return pl
}

func items(l *ast.List, src []byte, root, num string, depth int, out []Item) []Item {
	i := 0
	for li := l.FirstChild(); li != nil; li = li.NextSibling() {
		i++
		it := Item{Num: fmt.Sprintf("%s%d", num, i), Depth: depth}
		var kids *ast.List
		for b := li.FirstChild(); b != nil; b = b.NextSibling() {
			switch b := b.(type) {
			case *ast.List:
				if kids == nil {
					kids = b
				}
			case *ast.Paragraph, *ast.TextBlock:
				if it.Title == "" && it.Path == "" {
					it.Title, it.Note, it.Path, it.Frag = entry(b, src, root)
				}
			}
		}
		out = append(out, it)
		if kids != nil {
			out = items(kids, src, root, it.Num+".", depth+1, out)
		}
	}
	return out
}

// entry reads the first block of a list item: the first link is what to
// read, the text after it a note. Without a link the whole text is a title.
func entry(b ast.Node, src []byte, root string) (title, note, path, frag string) {
	var link *ast.Link
	var after strings.Builder
	for n := b.FirstChild(); n != nil; n = n.NextSibling() {
		if l, ok := n.(*ast.Link); ok && link == nil {
			link = l
			continue
		}
		if link != nil {
			after.WriteString(plainNode(n, src))
		}
	}
	if link == nil {
		return strings.TrimSpace(plain(b, src)), "", "", ""
	}
	dest := string(link.Destination)
	dest, frag, _ = strings.Cut(dest, "#")
	if u, err := url.PathUnescape(dest); err == nil {
		dest = u
	}
	note = strings.TrimSpace(after.String())
	note = strings.TrimSpace(strings.TrimLeft(note, "—–-:·"))
	return strings.TrimSpace(plain(link, src)), note, resolve(dest, root), frag
}

func resolve(p, root string) string {
	if p == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, p)
}

// plain is the text of n's inline children.
func plain(n ast.Node, src []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		sb.WriteString(plainNode(c, src))
	}
	return sb.String()
}

func plainNode(n ast.Node, src []byte) string {
	switch n := n.(type) {
	case *ast.Text:
		s := string(n.Segment.Value(src))
		if n.SoftLineBreak() || n.HardLineBreak() {
			s += " "
		}
		return s
	case *ast.String:
		return string(n.Value)
	case *ast.AutoLink:
		return string(n.URL(src))
	}
	return plain(n, src)
}

// lineStart is where the line holding n's first text starts.
func lineStart(src []byte, n ast.Node) int {
	at := len(src)
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && c.Type() == ast.TypeBlock && c.Lines().Len() > 0 {
			at = c.Lines().At(0).Start
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	for at > 0 && src[at-1] != '\n' {
		at--
	}
	return at
}

// lineEnd is just after the line holding n's last text.
func lineEnd(src []byte, n ast.Node) int {
	lines := n.Lines()
	if lines.Len() == 0 {
		return 0
	}
	at := lines.At(lines.Len() - 1).Stop
	for at < len(src) && src[at-1] != '\n' {
		at++
	}
	return at
}
