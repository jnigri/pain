// Package browser is the folder view: it scans a folder for Markdown files and
// shows them in three columns with reading progress.
package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"pain/internal/markdown"
	"pain/internal/plan"
	"pain/internal/store"
	"pain/internal/theme"
	"pain/internal/view"
)

// Node is a Markdown file or a folder holding some, in the folder view. It is
// also a reading plan, shown as a folder of rows, or one of its rows.
type Node struct {
	name   string
	Path   string
	Key    string // what progress is saved under: Path, or Path#Frag for part of a file
	Frag   string // the part of the file read: a heading anchor or a line range
	dir    bool
	parent *Node
	kids   []*Node
	// Plans and their rows. A plan's kids are its rows, flat and in order.
	plan    bool
	group   bool            // a row that only gathers the rows under it
	missing bool            // a row whose file or part is not found
	num     string          // the row's place in the outline: "2.1"
	note    string          // what the plan says about the row
	depth   int             // the row's nesting in the outline
	items   []*Node         // a plan's or group's rows to read
	desc    []markdown.Unit // a plan's description
	// Files only.
	units  []markdown.Unit
	stats  fileStats
	pos    store.Position
	hasPos bool
	// Words read and in total, and files done and in total. Folders sum
	// their children.
	read, total int
	done, files int
	// How the checkpoint questions went, the last time each section was
	// answered.
	score store.Score
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
		units := markdown.ParseIn(src, filepath.Dir(p))
		n.kids = append(n.kids, &Node{name: name, Path: p, Key: p, parent: n, units: units, stats: newFileStats(units)})
	}
	return n, nil
}

// Load scans path and puts the reading plans in its .reading-plans/ first.
func Load(path string) (*Node, error) {
	root, err := Scan(path, nil)
	if err != nil {
		return nil, err
	}
	var plans []*Node
	for _, pl := range plan.Load(path) {
		plans = append(plans, planNode(pl, root))
	}
	root.kids = append(plans, root.kids...)
	return root, nil
}

func planNode(pl plan.Plan, root *Node) *Node {
	p := &Node{name: pl.Title, Path: pl.File, Key: pl.File, dir: true, plan: true, parent: root, desc: pl.Desc}
	var up []*Node // the rows above, one per depth
	for _, it := range pl.Items {
		k := &Node{name: it.Title, Path: it.Path, Key: it.Path, Frag: it.Frag, plan: true, parent: p,
			num: it.Num, note: it.Note, depth: it.Depth}
		if it.Frag != "" {
			k.Key += "#" + it.Frag
		}
		up = up[:min(len(up), it.Depth)]
		if it.Group() {
			k.group = true
		} else if units, _, ok, err := markdown.Load(it.Path, it.Frag); err != nil || !ok {
			k.missing = true
		} else {
			k.units, k.stats = units, newFileStats(units)
			p.items = append(p.items, k)
			for _, g := range up {
				g.items = append(g.items, k)
			}
		}
		p.kids = append(p.kids, k)
		up = append(up, k)
	}
	return p
}

// counted is what n sums: a folder leaves out the plans, which only
// point at its files, and a plan or group counts the rows it can read.
func (n *Node) counted() []*Node {
	if n.plan {
		return n.items
	}
	var out []*Node
	for _, k := range n.kids {
		if !k.plan {
			out = append(out, k)
		}
	}
	return out
}

func (n *Node) readable() bool { return !n.dir && !n.group && !n.missing }

// refresh recounts what is read from the saved positions.
func (n *Node) refresh(all map[string]store.Position, scores map[string]store.FileScores) {
	if n.missing {
		return
	}
	if !n.dir && !n.group {
		n.score = scores[n.Key].Sum()
		n.pos, n.hasPos = all[n.Key]
		n.read, n.total = n.stats.read(n.pos, n.hasPos), n.stats.total()
		n.done, n.files = 0, 1
		if n.hasPos && n.pos.Done {
			n.done = 1
		}
		return
	}
	// A group's rows come after it: count them first.
	for _, k := range n.kids {
		if !k.group {
			k.refresh(all, scores)
		}
	}
	for _, k := range n.kids {
		if k.group {
			k.refresh(all, scores)
		}
	}
	n.read, n.total, n.done, n.files, n.score = 0, 0, 0, 0, store.Score{}
	for _, k := range n.counted() {
		n.score.Right += k.score.Right
		n.score.Total += k.score.Total
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

// eachFile calls fn on the files under n in folder order, until fn returns
// false. In a plan they are the rows to read, in plan order.
func (n *Node) eachFile(fn func(*Node) bool) bool {
	if !n.dir {
		return fn(n)
	}
	for _, k := range n.counted() {
		if !k.eachFile(fn) {
			return false
		}
	}
	return true
}

// Find is the file saved under key, in the folder first and then in its
// plans.
func (n *Node) Find(key string) *Node {
	var found *Node
	match := func(f *Node) bool {
		if f.Key == key {
			found = f
		}
		return found == nil
	}
	n.eachFile(match)
	for _, k := range n.kids {
		if k.plan && found == nil {
			k.eachFile(match)
		}
	}
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
	showStat bool
	active   bool // the folder is read actively: checkpoints end each section
}

func New(root *Node, last string) *Model {
	b := &Model{Root: root, dir: root, sel: map[*Node]int{}, Last: last, active: store.LoadPrefs().IsActive(root.Path)}
	b.Refresh()
	if n := root.Find(last); n != nil {
		b.Select(n)
	} else if n := b.NextUnread(nil); n != nil {
		b.Select(n)
	}
	return b
}

func (b *Model) Refresh() {
	b.Root.refresh(store.ReadProgress(), store.ReadScores())
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
// After a row of a plan it is the next row of that plan.
func (b *Model) NextUnread(after *Node) *Node {
	var first, found *Node
	seen := after == nil
	scope, walk := b.Root, b.Root.eachFile
	if after != nil && after.parent != nil && after.parent.plan {
		scope = after.parent
		// Walk every row, so that starting from a group works too.
		walk = func(fn func(*Node) bool) bool {
			for _, k := range scope.kids {
				if !fn(k) {
					return false
				}
			}
			return true
		}
	}
	walk(func(f *Node) bool {
		if f == after {
			seen = true
			return true
		}
		if f.readable() && f.done == 0 {
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
	if b.showKeys || b.showStat {
		b.showKeys, b.showStat = false, false
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
			} else if s.readable() {
				return s, false
			}
		}
	case "h", "left", "-":
		if b.dir.parent != nil {
			b.Select(b.dir)
		}
	case "n":
		after := b.selected()
		if after != nil && after.plan && after.dir && len(after.kids) > 0 {
			after = after.kids[0] // into the plan selected
			if after.readable() && after.done == 0 {
				b.Select(after)
				break
			}
		}
		if u := b.NextUnread(after); u != nil {
			b.Select(u)
		}
	case "r":
		if f := b.Root.Find(b.Last); f != nil {
			b.Select(f)
			return f, false
		}
	case "x":
		if s := b.selected(); s != nil && s.readable() {
			if s.done > 0 {
				store.ClearProgress(s.Key)
			} else {
				store.MarkDone(s.Key, s.stats.units())
			}
			b.Refresh()
		}
	case "s":
		b.showStat = true
	case "t":
		p := store.LoadPrefs()
		p.ToggleActive(b.Root.Path, b.Root.Path)
		if p.Save() == nil {
			b.active = p.IsActive(b.Root.Path)
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
	if b.showStat {
		return b.statsView()
	}
	const margin = 2
	w := b.W - 2*margin
	bodyH := max(b.H-4, 1) // header, two rules, footer
	sep := theme.Fg(theme.Muted).Render(" │ ")

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
			title := theme.Fg(theme.Purple).Bold(true).
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
	sb.WriteString(pad + theme.Fg(theme.Muted).Render(strings.Repeat("─", w)) + "\n")
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
	sb.WriteString(pad + theme.Fg(theme.Muted).Render(strings.Repeat("─", w)) + "\n")
	sb.WriteString(pad + b.footer(w))
	return sb.String()
}

func (b *Model) header(w int) string {
	r, files := b.Root, "files"
	left := theme.Fg(theme.Purple).Bold(true).Render(r.name)
	if b.dir.plan {
		r, files = b.dir, "items"
		left = theme.Fg(theme.Plan).Bold(true).Render(planGlyph + r.name)
	}
	if b.active {
		left += theme.Fg(theme.Green).Render("  ◆ active")
	}
	const barW = 20
	filled := barW * r.pct() / 100
	parts := []string{
		theme.Fg(theme.Foreground).Render(fmt.Sprintf("%s / %s words", thousands(r.read), thousands(r.total))),
		theme.Fg(theme.Yellow).Render(strings.Repeat("━", filled)) +
			theme.Fg(theme.Muted).Render(strings.Repeat("━", barW-filled)) +
			theme.Fg(theme.Muted).Render(fmt.Sprintf(" %3d%%", r.pct())),
		theme.Fg(theme.Foreground).Render(fmt.Sprintf("%d/%d %s", r.done, r.files, files)),
	}
	if r.score.Total > 0 {
		parts = append(parts, theme.Fg(scoreColor(r.score)).Render("◆ "+scoreText(r.score)))
	}
	// What matters least goes first when the screen is narrow.
	for len(parts) > 1 && lipgloss.Width(left)+3+lipgloss.Width(strings.Join(parts, "   ")) > w {
		parts = parts[1:]
	}
	if t := b.speed.TimeLeft(r.total - r.read); t != "" {
		parts = append(parts, theme.Fg(theme.Muted).Render(t))
	}
	right := strings.Join(parts, "   ")
	gap := max(w-lipgloss.Width(left)-lipgloss.Width(right), 3)
	return left + strings.Repeat(" ", gap) + right
}

func (b *Model) footer(w int) string {
	// The keys follow the entry selected.
	var parts []string
	if f := b.Root.Find(b.Last); f != nil && f.done == 0 && f != b.selected() {
		parts = append(parts, fmt.Sprintf("r resume %s (%d%%)", f.name, f.pct()))
	}
	switch s := b.selected(); {
	case s == nil, s.group:
	case s.missing:
		parts = append(parts, "not found")
	case s.plan && s.dir:
		parts = append(parts, "l open plan")
	case s.dir:
		parts = append(parts, "l open folder")
	case s.done > 0:
		parts = append(parts, "l read again", "x mark unread")
	case s.read > 0:
		parts = append(parts, "l carry on", "x mark read")
	default:
		parts = append(parts, "l read", "x mark read")
	}
	if b.dir.parent != nil {
		parts = append(parts, "h up")
	}
	if b.dir.plan {
		parts = append(parts, "n next in plan")
	} else {
		parts = append(parts, "n next unread")
	}
	active := "t active reading"
	if b.active {
		active = "t active off"
	}
	parts = append(parts, "s stats", active, "q quit", view.KeysHint)
	return theme.Fg(theme.Muted).Render(runewidth.Truncate(strings.Join(parts, " · "), w, "…"))
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

// planGlyph starts the name of a plan.
const planGlyph = "☰ "

func entryRow(n *Node, w int, sel, active bool) string {
	name, nameCol := n.name, theme.Foreground
	// A plan's row starts with its place in the outline.
	lead, leadCol := "", theme.Plan
	switch {
	case n.plan && n.dir:
		name, nameCol = planGlyph+name, theme.Plan
	case n.dir:
		name, nameCol = name+"/", theme.Blue
	case n.plan:
		lead = strings.Repeat("  ", n.depth) + n.num + " "
		if n.group {
			nameCol = theme.Plan
		}
	}
	mark, markCol := "", theme.Muted
	switch {
	case n.missing:
		mark, markCol, nameCol, leadCol = "missing", theme.Red, theme.Muted, theme.Muted
	case n.files > 0 && n.done == n.files:
		mark, markCol = "✓", theme.Green
	case n.read > 0:
		mark, markCol = fmt.Sprintf("%d%%", max(n.pct(), 1)), theme.Yellow
	}
	// The score of the checkpoint questions sits before the mark, when there
	// is room for it.
	score := ""
	if n.score.Total > 0 && w >= 24 {
		score = scoreText(n.score) + " "
	}
	side := runewidth.StringWidth(score) + runewidth.StringWidth(mark)
	lead = runewidth.Truncate(lead, max(w-2-side-2, 0), "")
	name = runewidth.Truncate(name, max(w-2-side-1-runewidth.StringWidth(lead), 1), "…")
	gap := max(w-2-runewidth.StringWidth(lead)-runewidth.StringWidth(name)-side, 1)

	st := lipgloss.NewStyle()
	nameSt := nameCol.On(st).Bold(n.group)
	switch {
	case sel && active:
		st = st.Background(theme.Surface())
		nameSt = theme.Foreground.On(st).Bold(true)
	case sel:
		nameSt = theme.Purple.On(nameSt).Bold(true)
	}
	return st.Render(" ") + leadCol.On(st).Render(lead) + nameSt.Render(name) + st.Render(strings.Repeat(" ", gap)) +
		scoreColor(n.score).On(st).Render(score) +
		markCol.On(st).Render(mark) + st.Render(" ")
}

// preview shows a folder's entries, or the file around where you stopped.
func (b *Model) preview(n *Node, w, h int) []string {
	if n == nil {
		return padColumn(nil, h)
	}
	switch {
	case n.plan && n.dir:
		return b.planPreview(n, w, h)
	case n.dir:
		return listColumn(n, nil, false, w, h)
	case n.missing:
		what := n.Path
		if n.Frag != "" {
			what += "#" + n.Frag
		}
		return padColumn([]string{theme.Fg(theme.Red).Render("not found"), "",
			theme.Fg(theme.Muted).Render(runewidth.Truncate(what, w, "…"))}, h)
	case n.group:
		// The rows under it.
		var lines []string
		for _, k := range n.parent.kids {
			if strings.HasPrefix(k.num, n.num+".") && len(lines) < h {
				lines = append(lines, entryRow(k, w, false, false))
			}
		}
		return padColumn(lines, h)
	}
	var lines []string
	if n.note != "" {
		for _, l := range strings.Split(lipgloss.NewStyle().Width(w).Render(n.note), "\n") {
			lines = append(lines, theme.Fg(theme.Plan).Italic(true).Render(l))
		}
		lines = append(lines, "")
	}
	status := fmt.Sprintf("%s words · not started", thousands(n.total))
	switch {
	case n.done > 0:
		status = fmt.Sprintf("%s words · ✓ read", thousands(n.total))
	case n.read > 0:
		status = fmt.Sprintf("%s / %s words · %d%%", thousands(n.read), thousands(n.total), n.pct())
	}
	if n.score.Total > 0 {
		status += " · questions " + scoreText(n.score)
	}
	lines = append(lines, theme.Fg(theme.Muted).Render(status), "")
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

// planPreview shows a plan's description, then its rows.
func (b *Model) planPreview(n *Node, w, h int) []string {
	status := fmt.Sprintf("%d/%d items read", n.done, n.files)
	if n.total > 0 {
		status += fmt.Sprintf(" · %s / %s words · %d%%", thousands(n.read), thousands(n.total), n.pct())
	}
	lines := []string{theme.Fg(theme.Muted).Render(status), ""}
	for i := range n.desc {
		ul, _ := view.RenderUnit(&n.desc[i], w, nil, false)
		lines = append(lines, ul...)
	}
	if len(n.desc) > 0 {
		lines = append(lines, "")
	}
	for _, k := range n.kids {
		lines = append(lines, entryRow(k, w, false, false))
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

func scoreText(s store.Score) string { return fmt.Sprintf("%d/%d", s.Right, s.Total) }

// scoreColor is green when every question was answered right.
func scoreColor(s store.Score) theme.Color {
	if s.Right == s.Total {
		return theme.Green
	}
	return theme.Teal
}

// statsView compares what was typed and answered in this folder with
// everything typed in pain.
func (b *Model) statsView() string {
	here, all := b.speed.Under(b.Root.Path), b.speed.Tally
	wpm := func(v float64) string {
		if v <= 0 {
			return "–"
		}
		return fmt.Sprintf("%.0f wpm", v)
	}
	right := func(t store.Tally) string {
		if t.Answered == 0 {
			return "–"
		}
		return fmt.Sprintf("%d of %d (%d%%)", t.Right, t.Answered, t.Right*100/t.Answered)
	}
	clock := func(t store.Tally) string { return store.Clock(time.Duration(t.Seconds * float64(time.Second))) }
	rows := [][3]string{
		{"", b.Root.name, "everywhere"},
		{"words typed", thousands(here.Words), thousands(all.Words)},
		{"time typing", clock(here), clock(all)},
		{"average speed", wpm(here.WPM()), wpm(all.WPM())},
		{"top speed", wpm(here.BestWPM), wpm(all.BestWPM)},
		{"questions right", right(here), right(all)},
		{"explanations", fmt.Sprint(here.Explained), fmt.Sprint(all.Explained)},
	}
	if b.Root.score.Total > 0 {
		rows = append(rows, [3]string{"latest scores", scoreText(b.Root.score), ""})
	}
	var ws [3]int
	for _, r := range rows {
		for i, c := range r {
			ws[i] = max(ws[i], runewidth.StringWidth(c))
		}
	}
	lines := []string{theme.Fg(theme.Purple).Bold(true).Render("stats"), ""}
	for k, r := range rows {
		st := theme.Fg(theme.Foreground)
		if k == 0 {
			st = theme.Fg(theme.Muted)
		}
		l := theme.Fg(theme.Foreground).Render(r[0]) + strings.Repeat(" ", ws[0]-runewidth.StringWidth(r[0]))
		for i := 1; i < 3; i++ {
			l += strings.Repeat(" ", 3+ws[i]-runewidth.StringWidth(r[i])) + st.Render(r[i])
		}
		lines = append(lines, l)
	}
	lines = append(lines, "", theme.Fg(theme.Muted).Render("Top speed is the fastest run of 20 words. Questions count every try."),
		"", theme.Fg(theme.Muted).Render("any key to close"))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Edge()).Padding(1, 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(b.W, b.H, lipgloss.Center, lipgloss.Center, box)
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
	{Key: "l / → / enter", Action: "open the folder, reading plan or file"},
	{Key: "h / ← / -", Action: "go up a folder"},
	{Key: "g / G", Action: "go to the first / last entry"},
	{Key: "n", Action: "select the next file not read yet; in a plan, its next row"},
	{Key: "r", Action: "reopen the last file you read"},
	{Key: "x", Action: "mark the file read, or unread"},
	{Key: "s", Action: "stats: words typed, speed and questions, here and everywhere"},
	{Key: "t", Action: "active reading on or off: a checkpoint ends each section"},
	{Key: "q / esc / ctrl+c", Action: "quit"},
}
