// Package browser is the folder view: it scans a folder for Markdown files and
// shows them in three columns with reading progress.
package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"pain/internal/markdown"
	"pain/internal/store"
	"pain/internal/theme"
	"pain/internal/view"
)

// Node is a Markdown file or a folder holding some, in the folder view.
type Node struct {
	name   string
	Path   string
	dir    bool
	parent *Node
	kids   []*Node
	// Files only.
	units  []markdown.Unit
	stats  fileStats
	pos    store.Position
	hasPos bool
	// Words read and in total, and files done and in total. Folders sum
	// their children.
	read, total int
	done, files int
}

// Scan reads the Markdown files under path. Hidden entries and folders
// without Markdown are left out.
func Scan(path string, parent *Node) (*Node, error) {
	n := &Node{name: filepath.Base(path), Path: path, dir: true, parent: parent}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		p := filepath.Join(path, name)
		if e.IsDir() {
			if k, err := Scan(p, n); err == nil && len(k.kids) > 0 {
				n.kids = append(n.kids, k)
			}
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		src, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		units := markdown.Parse(src)
		n.kids = append(n.kids, &Node{name: name, Path: p, parent: n, units: units, stats: newFileStats(units)})
	}
	return n, nil
}

// refresh recounts what is read from the saved positions.
func (n *Node) refresh(all map[string]store.Position) {
	if !n.dir {
		n.pos, n.hasPos = all[n.Path]
		n.read, n.total = n.stats.read(n.pos, n.hasPos), n.stats.total()
		n.done, n.files = 0, 1
		if n.hasPos && n.pos.Done {
			n.done = 1
		}
		return
	}
	n.read, n.total, n.done, n.files = 0, 0, 0, 0
	for _, k := range n.kids {
		k.refresh(all)
		n.read += k.read
		n.total += k.total
		n.done += k.done
		n.files += k.files
	}
}

func (n *Node) pct() int {
	if n.total == 0 {
		return 0
	}
	return n.read * 100 / n.total
}

// eachFile calls fn on the files under n in folder order, until fn returns false.
func (n *Node) eachFile(fn func(*Node) bool) bool {
	if !n.dir {
		return fn(n)
	}
	for _, k := range n.kids {
		if !k.eachFile(fn) {
			return false
		}
	}
	return true
}

func (n *Node) Find(path string) *Node {
	var found *Node
	n.eachFile(func(f *Node) bool {
		if f.Path == path {
			found = f
		}
		return found == nil
	})
	return found
}

// Model is the folder view: the parent folder, the current folder and a
// preview of the selected entry, side by side as in mini.files.
type Model struct {
	Root     *Node
	dir      *Node // the folder in the middle column
	sel      map[*Node]int
	Last     string // the file opened last, to resume
	speed    store.TypingStats
	W, H     int
	showKeys bool
}

func New(root *Node, last string) *Model {
	b := &Model{Root: root, dir: root, sel: map[*Node]int{}, Last: last}
	b.Refresh()
	if n := root.Find(last); n != nil {
		b.Select(n)
	} else if n := b.NextUnread(nil); n != nil {
		b.Select(n)
	}
	return b
}

func (b *Model) Refresh() {
	b.Root.refresh(store.ReadProgress())
	b.speed = store.LoadTypingStats()
}

func (b *Model) selected() *Node {
	if len(b.dir.kids) == 0 {
		return nil
	}
	i := min(max(b.sel[b.dir], 0), len(b.dir.kids)-1)
	return b.dir.kids[i]
}

// Select shows n's folder in the middle column with n selected, and
// selects each folder on the way up in its parent.
func (b *Model) Select(n *Node) {
	b.dir = n.parent
	for c := n; c.parent != nil; c = c.parent {
		for i, k := range c.parent.kids {
			if k == c {
				b.sel[c.parent] = i
			}
		}
	}
}

// NextUnread is the first file after `after` in folder order that is not
// read to the end, wrapping around. With after nil it starts from the top.
func (b *Model) NextUnread(after *Node) *Node {
	var first, found *Node
	seen := after == nil
	b.Root.eachFile(func(f *Node) bool {
		if f == after {
			seen = true
			return true
		}
		if f.done == 0 {
			if first == nil {
				first = f
			}
			if seen {
				found = f
				return false
			}
		}
		return true
	})
	if found == nil {
		found = first
	}
	return found
}

// Update handles a key and returns the file to open, if any.
func (b *Model) Update(msg tea.KeyMsg) (open *Node, quit bool) {
	if b.showKeys {
		b.showKeys = false
		return nil, false
	}
	n := len(b.dir.kids)
	switch msg.String() {
	case "ctrl+_":
		b.showKeys = true
	case "j", "down":
		b.sel[b.dir] = min(b.sel[b.dir]+1, n-1)
	case "k", "up":
		b.sel[b.dir] = max(b.sel[b.dir]-1, 0)
	case "g", "home":
		b.sel[b.dir] = 0
	case "G", "end":
		b.sel[b.dir] = n - 1
	case "l", "right", "enter":
		if s := b.selected(); s != nil {
			if s.dir {
				b.dir = s
			} else {
				return s, false
			}
		}
	case "h", "left", "-":
		if b.dir.parent != nil {
			b.Select(b.dir)
		}
	case "n":
		if u := b.NextUnread(b.selected()); u != nil {
			b.Select(u)
		}
	case "r":
		if f := b.Root.Find(b.Last); f != nil {
			b.Select(f)
			return f, false
		}
	case "x":
		if s := b.selected(); s != nil && !s.dir {
			if s.done > 0 {
				store.ClearProgress(s.Path)
			} else {
				store.MarkDone(s.Path, s.stats.units())
			}
			b.Refresh()
		}
	case "q", "esc", "ctrl+c":
		return nil, true
	}
	return nil, false
}

func (b *Model) View() string {
	if b.W == 0 {
		return ""
	}
	if b.showKeys {
		return view.KeysBox(b.W, b.H, "keys · files", browseKeys)
	}
	const margin = 2
	w := b.W - 2*margin
	bodyH := max(b.H-4, 1) // header, two rules, footer
	sep := theme.Fg(theme.Overlay0).Render(" │ ")

	// The parent column goes first when the screen is narrow.
	var cols [][]string
	var widths []int
	pw := 0
	if w >= 70 {
		pw = min(max(w/5, 14), 30)
	}
	mw := min(max(w*3/10, 20), 44)
	vw := max(w-pw-mw-2*3, 10)
	if pw > 0 {
		if b.dir.parent != nil {
			cols = append(cols, listColumn(b.dir.parent, b.dir, false, pw, bodyH))
		} else {
			title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).
				Render(runewidth.Truncate(" "+b.Root.name+"/", pw, "…"))
			cols = append(cols, padColumn([]string{title}, bodyH))
		}
		widths = append(widths, pw)
	}
	cols = append(cols, listColumn(b.dir, b.selected(), true, mw, bodyH))
	widths = append(widths, mw)
	cols = append(cols, b.preview(b.selected(), vw, bodyH))
	widths = append(widths, vw)

	pad := strings.Repeat(" ", margin)
	var sb strings.Builder
	sb.WriteString(pad + b.header(w) + "\n")
	sb.WriteString(pad + theme.Fg(theme.Overlay0).Render(strings.Repeat("─", w)) + "\n")
	for i := 0; i < bodyH; i++ {
		sb.WriteString(pad)
		for c, col := range cols {
			if c > 0 {
				sb.WriteString(sep)
			}
			l := col[i]
			if c < len(cols)-1 {
				l += strings.Repeat(" ", max(widths[c]-lipgloss.Width(l), 0))
			}
			sb.WriteString(l)
		}
		sb.WriteString("\n")
	}
	sb.WriteString(pad + theme.Fg(theme.Overlay0).Render(strings.Repeat("─", w)) + "\n")
	sb.WriteString(pad + b.footer(w))
	return sb.String()
}

func (b *Model) header(w int) string {
	r := b.Root
	left := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render(r.name)
	const barW = 20
	filled := barW * r.pct() / 100
	parts := []string{
		theme.Fg(theme.Subtext1).Render(fmt.Sprintf("%s / %s words", thousands(r.read), thousands(r.total))),
		theme.Fg(theme.Yellow).Render(strings.Repeat("━", filled)) +
			theme.Fg(theme.Overlay0).Render(strings.Repeat("━", barW-filled)) +
			theme.Fg(theme.Subtext0).Render(fmt.Sprintf(" %3d%%", r.pct())),
		theme.Fg(theme.Subtext1).Render(fmt.Sprintf("%d/%d files", r.done, r.files)),
	}
	if t := b.speed.TimeLeft(r.total - r.read); t != "" {
		parts = append(parts, theme.Fg(theme.Subtext0).Render(t))
	}
	right := strings.Join(parts, "   ")
	gap := max(w-lipgloss.Width(left)-lipgloss.Width(right), 3)
	return left + strings.Repeat(" ", gap) + right
}

func (b *Model) footer(w int) string {
	help := "l open · h up · n next unread · x mark read · q quit · " + view.KeysHint
	if f := b.Root.Find(b.Last); f != nil {
		help = fmt.Sprintf("r resume %s (%d%%) · ", f.name, f.pct()) + help
	}
	return theme.Fg(theme.Overlay0).Render(runewidth.Truncate(help, w, "…"))
}

// listColumn draws the entries of folder dir, with sel highlighted: as the
// selection when active, else as the way to the next column.
func listColumn(dir, sel *Node, active bool, w, h int) []string {
	at := -1
	for i, k := range dir.kids {
		if k == sel {
			at = i
		}
	}
	start := min(max(at-h/2, 0), max(len(dir.kids)-h, 0))
	var lines []string
	for i := start; i < len(dir.kids) && len(lines) < h; i++ {
		lines = append(lines, entryRow(dir.kids[i], w, i == at, active))
	}
	return padColumn(lines, h)
}

func entryRow(n *Node, w int, sel, active bool) string {
	name, nameCol := n.name, theme.Subtext1
	if n.dir {
		name, nameCol = name+"/", theme.Blue
	}
	mark, markCol := "", theme.Overlay2
	switch {
	case n.files > 0 && n.done == n.files:
		mark, markCol = "✓", theme.Green
	case n.read > 0:
		mark, markCol = fmt.Sprintf("%d%%", max(n.pct(), 1)), theme.Yellow
	}
	name = runewidth.Truncate(name, max(w-2-runewidth.StringWidth(mark)-1, 1), "…")
	gap := max(w-2-runewidth.StringWidth(name)-runewidth.StringWidth(mark), 1)

	st := lipgloss.NewStyle()
	nameSt := st.Foreground(lipgloss.Color(nameCol))
	switch {
	case sel && active:
		st = st.Background(lipgloss.Color(theme.Surface0))
		nameSt = st.Foreground(lipgloss.Color(theme.Foreground)).Bold(true)
	case sel:
		nameSt = nameSt.Bold(true).Foreground(lipgloss.Color(theme.Purple))
	}
	return st.Render(" ") + nameSt.Render(name) + st.Render(strings.Repeat(" ", gap)) +
		st.Foreground(lipgloss.Color(markCol)).Render(mark) + st.Render(" ")
}

// preview shows a folder's entries, or the file around where you stopped.
func (b *Model) preview(n *Node, w, h int) []string {
	if n == nil {
		return padColumn(nil, h)
	}
	if n.dir {
		return listColumn(n, nil, false, w, h)
	}
	status := fmt.Sprintf("%s words · not started", thousands(n.total))
	switch {
	case n.done > 0:
		status = fmt.Sprintf("%s words · ✓ read", thousands(n.total))
	case n.read > 0:
		status = fmt.Sprintf("%s / %s words · %d%%", thousands(n.read), thousands(n.total), n.pct())
	}
	lines := []string{theme.Fg(theme.Subtext0).Render(status), ""}
	at := 0
	if n.hasPos && !n.pos.Done && n.read > 0 {
		at = n.pos.Unit
	}
	for i := max(at-2, 0); i < len(n.units) && len(lines) < h; i++ {
		ul, _ := view.RenderUnit(&n.units[i], w, nil, i < at || n.done > 0)
		lines = append(lines, ul...)
	}
	return padColumn(lines, h)
}

// padColumn cuts or fills lines to exactly h.
func padColumn(lines []string, h int) []string {
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

var browseKeys = []view.KeyRow{
	{Key: "j / k / ↓ / ↑", Action: "move the selection"},
	{Key: "l / → / enter", Action: "open the folder or file"},
	{Key: "h / ← / -", Action: "go up a folder"},
	{Key: "g / G", Action: "go to the first / last entry"},
	{Key: "n", Action: "select the next file not read yet"},
	{Key: "r", Action: "reopen the last file you read"},
	{Key: "x", Action: "mark the file read, or unread"},
	{Key: "q / esc / ctrl+c", Action: "quit"},
}
