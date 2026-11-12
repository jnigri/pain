// Package markdown turns a Markdown file into units: the blocks you type one
// at a time, each a run of styled cells. It also holds the text rules shared
// by typing and drawing: character folding and matching, word wrapping and
// sentence bounds. It knows nothing about the terminal.
package markdown

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/unicode/norm"
)

// Attr holds inline styling for a single cell.
type Attr uint16

const (
	Bold Attr = 1 << iota
	Italic
	InlineCode
	Link
	Strike
	TableHeader // table header cell
	Mark        // decoration: table separators, task boxes, code language label
)

// Cell is one rune of a unit. Skip cells are shown but never typed.
type Cell struct {
	R    rune
	Attr Attr
	Skip bool
}

type Kind int

const (
	Para Kind = iota
	Heading
	CodeLine
	Rule
	Spacer
	Image // a picture, shown but never typed
)

// Unit is one thing to type: a paragraph, heading, list item, code line or table row.
type Unit struct {
	Kind      Kind
	Level     int // heading level
	Cells     []Cell
	Indent    int    // columns before the prefix
	Prefix    string // bullet or number, shown on the first line only
	ListDepth int    // nesting of the list the prefix belongs to, 0 for the outermost
	Ordered   bool   // prefix is a number rather than a bullet
	Quote     int    // blockquote depth
	Image     string // for an Image, the file to show; Cells hold its alt text
}

func (u *Unit) Typeable() bool {
	if u.Kind == Rule || u.Kind == Spacer || u.Kind == Image {
		return false
	}
	for _, c := range u.Cells {
		if !c.Skip {
			return true
		}
	}
	return false
}

type ctx struct {
	indent    int
	quote     int
	depth     int // list nesting
	prefix    string
	listDepth int
	ordered   bool
}

type builder struct {
	src   []byte
	dir   string
	units []Unit
}

func Parse(src []byte) []Unit { return ParseIn(src, "") }

// ParseIn parses a file that sits in dir, which is where its relative image
// paths start from.
func ParseIn(src []byte, dir string) []Unit {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	doc := md.Parser().Parse(text.NewReader(src))
	b := &builder{src: src, dir: dir}
	b.blocks(doc, ctx{})
	for len(b.units) > 0 && b.units[len(b.units)-1].Kind == Spacer {
		b.units = b.units[:len(b.units)-1]
	}
	return b.units
}

func (b *builder) add(u Unit, c ctx) {
	u.Indent, u.Quote, u.Prefix, u.ListDepth, u.Ordered = c.indent, c.quote, c.prefix, c.listDepth, c.ordered
	b.units = append(b.units, u)
}

func (b *builder) spacer(c ctx) {
	if len(b.units) == 0 || b.units[len(b.units)-1].Kind == Spacer {
		return
	}
	b.units = append(b.units, Unit{Kind: Spacer, Quote: c.quote})
}

func (b *builder) blocks(n ast.Node, c ctx) {
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		b.block(ch, c)
		// The bullet belongs to the first block of a list item only.
		if c.prefix != "" {
			c.indent += runewidth.StringWidth(c.prefix)
			c.prefix = ""
		}
	}
}

func (b *builder) block(n ast.Node, c ctx) {
	switch n := n.(type) {
	case *ast.Heading:
		b.add(Unit{Kind: Heading, Level: n.Level, Cells: skipNumbering(tidy(b.inlines(n, 0)))}, c)
		b.spacer(c)
	case *ast.Paragraph:
		b.text(n, c)
		b.spacer(c)
	case *ast.TextBlock: // tight list item: no gap after it
		b.text(n, c)
	case *ast.List:
		num := n.Start
		for item := n.FirstChild(); item != nil; item = item.NextSibling() {
			ic := c
			ic.depth = c.depth + 1
			ic.listDepth, ic.ordered = c.depth, n.IsOrdered()
			if n.IsOrdered() {
				ic.prefix = fmt.Sprintf("%d. ", num)
				num++
			} else {
				bullets := []string{"• ", "◦ ", "▪ "}
				ic.prefix = bullets[c.depth%3]
			}
			b.blocks(item, ic)
		}
		if c.depth == 0 {
			b.spacer(c)
		}
	case *ast.Blockquote:
		qc := c
		qc.quote++
		b.blocks(n, qc)
		if last := len(b.units) - 1; last >= 0 && b.units[last].Kind == Spacer {
			b.units[last].Quote = c.quote
		}
		b.spacer(c)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		c.indent += runewidth.StringWidth(c.prefix)
		c.prefix = ""
		if fc, ok := n.(*ast.FencedCodeBlock); ok {
			if lang := string(fc.Language(b.src)); lang != "" {
				b.add(Unit{Kind: CodeLine, Cells: Cells(lang, Mark, true)}, c)
			}
		}
		lines := n.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			line := strings.TrimRight(string(seg.Value(b.src)), "\r\n")
			line = strings.ReplaceAll(line, "\t", "    ")
			cs := Cells(line, 0, false)
			for j := range cs { // leading indentation is filled in for you
				if cs[j].R != ' ' {
					break
				}
				cs[j].Skip = true
			}
			b.add(Unit{Kind: CodeLine, Cells: cs}, c)
		}
		b.spacer(c)
	case *ast.ThematicBreak:
		b.add(Unit{Kind: Rule}, c)
		b.spacer(c)
	case *east.Table:
		b.table(n, c)
		b.spacer(c)
	case *ast.HTMLBlock:
		// Raw HTML is not rendered, apart from its images.
		var sb strings.Builder
		for i := 0; i < n.Lines().Len(); i++ {
			seg := n.Lines().At(i)
			sb.Write(seg.Value(b.src))
		}
		if imgs := b.htmlImages(sb.String()); len(imgs) > 0 {
			for _, u := range imgs {
				b.add(u, c)
				c.indent, c.prefix = c.indent+runewidth.StringWidth(c.prefix), ""
			}
			b.spacer(c)
		}
	default:
		if n.HasChildren() {
			b.blocks(n, c)
		}
	}
}

// text adds a paragraph, then the pictures it holds. A paragraph of nothing
// but pictures adds only those.
func (b *builder) text(n ast.Node, c ctx) {
	imgs, only := b.images(n)
	if !only {
		b.add(Unit{Kind: Para, Cells: tidy(b.inlines(n, 0))}, c)
		c.indent, c.prefix = c.indent+runewidth.StringWidth(c.prefix), ""
	}
	for _, u := range imgs {
		b.add(u, c)
		c.indent, c.prefix = c.indent+runewidth.StringWidth(c.prefix), ""
	}
}

// images finds the pictures in a paragraph that are files on this machine.
// The others stay in the text, as their alt text. only tells whether the
// paragraph holds nothing else.
func (b *builder) images(n ast.Node) (imgs []Unit, only bool) {
	only = true
	_ = ast.Walk(n, func(ch ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch ch := ch.(type) {
		case *ast.Image:
			if u, ok := b.image(string(ch.Destination), tidy(b.inlines(ch, Italic))); ok {
				imgs = append(imgs, u)
			} else {
				only = false
			}
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			var sb strings.Builder
			for i := 0; i < ch.Segments.Len(); i++ {
				seg := ch.Segments.At(i)
				sb.Write(seg.Value(b.src))
			}
			imgs = append(imgs, b.htmlImages(sb.String())...)
		case *ast.Text:
			if strings.TrimSpace(string(ch.Segment.Value(b.src))) != "" {
				only = false
			}
		case *ast.String, *ast.CodeSpan, *ast.AutoLink:
			only = false
		}
		return ast.WalkContinue, nil
	})
	return imgs, only && len(imgs) > 0
}

var (
	imgTag  = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	srcAttr = regexp.MustCompile(`(?is)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	altAttr = regexp.MustCompile(`(?is)\balt\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// htmlImages finds the <img> tags of raw HTML that show a file on this machine.
func (b *builder) htmlImages(html string) []Unit {
	var out []Unit
	for _, tag := range imgTag.FindAllString(html, -1) {
		src := srcAttr.FindStringSubmatch(tag)
		if src == nil {
			continue
		}
		alt := ""
		if m := altAttr.FindStringSubmatch(tag); m != nil {
			alt = m[1] + m[2]
		}
		if u, ok := b.image(src[1]+src[2], tidy(Cells(alt, Italic, false))); ok {
			out = append(out, u)
		}
	}
	return out
}

// image is the unit for a picture at dest, as written in the file. ok is
// false for one that is not a file on this machine.
func (b *builder) image(dest string, alt []Cell) (Unit, bool) {
	if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "//") || strings.HasPrefix(dest, "data:") {
		return Unit{}, false
	}
	if i := strings.IndexAny(dest, "?#"); i >= 0 {
		dest = dest[:i]
	}
	if p, err := url.PathUnescape(dest); err == nil {
		dest = p
	}
	dest = filepath.FromSlash(dest)
	if !filepath.IsAbs(dest) && b.dir != "" {
		dest = filepath.Join(b.dir, dest)
	}
	for i := range alt {
		alt[i].Skip = true
	}
	return Unit{Kind: Image, Image: dest, Cells: alt}, true
}

func (b *builder) table(t *east.Table, c ctx) {
	var rows [][][]Cell
	for row := t.FirstChild(); row != nil; row = row.NextSibling() {
		var r [][]Cell
		_, header := row.(*east.TableHeader)
		for tc := row.FirstChild(); tc != nil; tc = tc.NextSibling() {
			var f Attr
			if header {
				f = TableHeader
			}
			r = append(r, tidy(b.inlines(tc, f)))
		}
		rows = append(rows, r)
	}
	var widths []int
	for _, r := range rows {
		for i, cs := range r {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], Width(cs))
		}
	}
	for ri, r := range rows {
		var line []Cell
		for i, cs := range r {
			line = append(line, cs...)
			if i < len(r)-1 {
				line = append(line, Cells(strings.Repeat(" ", widths[i]-Width(cs)), 0, true)...)
				// The space after the bar is typed, so a row reads "cell cell".
				line = append(line, Cells(" │", Mark, true)...)
				line = append(line, Cell{R: ' '})
			}
		}
		b.add(Unit{Kind: Para, Cells: line}, c)
		if ri == 0 {
			var parts []string
			for _, w := range widths {
				parts = append(parts, strings.Repeat("─", w))
			}
			b.add(Unit{Kind: Para, Cells: Cells(strings.Join(parts, "─┼─"), Mark, true)}, c)
		}
	}
}

func (b *builder) inlines(n ast.Node, f Attr) []Cell {
	var out []Cell
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch ch := ch.(type) {
		case *ast.Text:
			out = append(out, Cells(string(ch.Segment.Value(b.src)), f, false)...)
			if ch.SoftLineBreak() || ch.HardLineBreak() {
				out = append(out, Cell{R: ' ', Attr: f})
			}
		case *ast.String:
			out = append(out, Cells(string(ch.Value), f, false)...)
		case *ast.CodeSpan:
			out = append(out, b.inlines(ch, f|InlineCode)...)
		case *ast.Emphasis:
			if ch.Level >= 2 {
				out = append(out, b.inlines(ch, f|Bold)...)
			} else {
				out = append(out, b.inlines(ch, f|Italic)...)
			}
		case *ast.Link:
			out = append(out, b.inlines(ch, f|Link)...)
		case *ast.AutoLink:
			out = append(out, Cells(string(ch.URL(b.src)), f|Link, false)...)
		case *ast.Image:
			// One shown as a picture is not also in the text.
			if _, ok := b.image(string(ch.Destination), nil); !ok {
				out = append(out, b.inlines(ch, f|Italic|Link)...)
			}
		case *ast.RawHTML:
		case *east.Strikethrough:
			out = append(out, b.inlines(ch, f|Strike)...)
		case *east.TaskCheckBox:
			box := "☐ "
			if ch.IsChecked {
				box = "☑ "
			}
			out = append(out, Cells(box, f|Mark, true)...)
		default:
			out = append(out, b.inlines(ch, f)...)
		}
	}
	return out
}

func Cells(s string, f Attr, skip bool) []Cell {
	var out []Cell
	for _, r := range s {
		if r == '\t' || r == '\n' {
			r = ' '
		}
		out = append(out, Cell{R: r, Attr: f, Skip: skip || untypeable(r)})
	}
	return out
}

// tidy collapses runs of spaces outside code and trims both ends.
func tidy(cs []Cell) []Cell {
	var out []Cell
	for _, c := range cs {
		if c.R == ' ' && c.Attr&InlineCode == 0 && !c.Skip && (len(out) == 0 || out[len(out)-1].R == ' ') {
			continue
		}
		out = append(out, c)
	}
	for len(out) > 0 && out[len(out)-1].R == ' ' && !out[len(out)-1].Skip {
		out = out[:len(out)-1]
	}
	return out
}

// skipNumbering auto-fills section numbers like "1. " or "2.3 " that start a heading.
func skipNumbering(cs []Cell) []Cell {
	i := 0
	for i < len(cs) && (unicode.IsDigit(cs[i].R) || cs[i].R == '.') {
		i++
	}
	if i == 0 || i >= len(cs) || cs[i].R != ' ' || !unicode.IsDigit(cs[0].R) {
		return cs
	}
	for j := 0; j <= i; j++ {
		cs[j].Skip = true
	}
	return cs
}

func Width(cs []Cell) int {
	w := 0
	for _, c := range cs {
		w += runewidth.RuneWidth(c.R)
	}
	return w
}

// fold maps a rune to what you would type for it on a plain keyboard.
func fold(r rune) rune {
	switch r {
	case '‘', '’', '‚', '′':
		return '\''
	case '“', '”', '„', '″', '«', '»':
		return '"'
	case '–', '—', '−':
		return '-'
	case '…':
		return '.'
	case ' ', ' ', ' ':
		return ' '
	}
	if r > unicode.MaxASCII {
		if d := []rune(norm.NFD.String(string(r))); len(d) > 0 && d[0] <= unicode.MaxASCII {
			return d[0]
		}
	}
	return r
}

// untypeable runes (emoji, arrows, box drawing...) are auto-filled.
func untypeable(r rune) bool {
	return fold(r) > unicode.MaxASCII && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func Matches(typed, want rune, caseSensitive bool) bool {
	if typed == want || fold(typed) == fold(want) {
		return true
	}
	return !caseSensitive && unicode.ToLower(fold(typed)) == unicode.ToLower(fold(want))
}

func IsAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
