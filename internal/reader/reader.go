// Package reader is the file view: typing through a document, asking Claude
// about a block, and the preferences screen.
package reader

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"pain/internal/markdown"
	"pain/internal/store"
	"pain/internal/theme"
	"pain/internal/view"
)

const maxWidth = 88

// maxKeyGap caps the time counted between two keys, so a pause does not
// count as reading time.
const maxKeyGap = 10 * time.Second

// burstWords is the run of words the top typing speed is measured over.
const burstWords = 20

// maxWPM is the fastest run still counted as typing.
const maxWPM = 300

// scope is what the underline covers, and so what ctrl+l and ctrl+h move by
// and what ctrl+a and ctrl+e are about. Tab widens it.
type scope int

const (
	scopeSentence scope = iota
	scopeBlock
	scopeSection
)

var scopeNames = [...]string{"sentence", "block", "section"}

type cached struct {
	done  bool
	lines []string
}

type Model struct {
	Path     string
	src      []byte
	units    []markdown.Unit
	cur      int
	typed    []rune
	total    int // typeable units
	w, h     int
	cache    map[int]cached
	Finished bool
	prefs    store.Prefs
	showPref bool
	showKeys bool
	sel      scope
	secs     []section // the file cut into sections, for the section scope and checkpoints
	prefSel  int
	prefErr  error
	pick     *picker // model choice, over everything else while open
	ask      *askState
	asked    map[int]bool // units with a conversation
	scroll   int          // lines the view is moved down from the cursor, up when negative
	// Folder is set when the file was opened from the folder view: esc goes
	// back to it, and finishing returns there.
	Folder bool
	// What was typed and answered this session, saved to stats.json on
	// leaving. typedSecs is the time spent typing, and wordAt its value at
	// each word typed, for the top speed.
	lastKey   time.Time
	words     int
	typedSecs float64
	wordAt    []float64
	sess      store.Tally // top speed, questions and explanations
	// Scope is what ctrl+t turns active reading on for: the folder the file
	// was opened from. Empty, it is the file alone.
	Scope  string
	check  *checks    // nil unless the file is read actively
	quiz   *quizState // the checkpoint asking questions, while open
	armID  int        // counts the choices armed, to drop the ticks of earlier ones
	notice string     // shown in place of the help until the next key
	closed bool       // the folder view was asked to take over
}

// CloseMsg asks the folder view to take over again.
type CloseMsg struct{ Finished bool }

func closeDoc(finished bool) tea.Cmd {
	return func() tea.Msg { return CloseMsg{finished} }
}

// leave saves where you are and how fast you typed.
func (m *Model) leave() {
	if !m.Finished {
		store.SaveProgress(m.Path, m.cur, len(m.units))
	}
	store.AddStats(m.Path, m.tally())
	m.words, m.typedSecs, m.wordAt, m.sess = 0, 0, nil, store.Tally{}
	m.stopGen()
}

// tally is this session so far.
func (m *Model) tally() store.Tally {
	t := m.sess
	t.Words, t.Seconds = m.words, m.typedSecs
	return t
}

// tick counts the time since the last typing key, up to maxKeyGap.
func (m *Model) tick() {
	now := time.Now()
	if !m.lastKey.IsZero() {
		m.typedSecs += min(now.Sub(m.lastKey), maxKeyGap).Seconds()
	}
	m.lastKey = now
}

func New(path string, src []byte, units []markdown.Unit, start int, p store.Prefs) *Model {
	m := &Model{Path: path, src: src, units: units, cache: map[int]cached{}, prefs: p, secs: splitSections(units)}
	m.countTotal()
	m.loadAsked()
	m.cur = start - 1
	m.next()
	if p.IsActive(path) {
		m.setupChecks()
	}
	return m
}

// needsTyping tells whether unit i stops the cursor. With skip punctuation on,
// units holding no letters or digits are passed over.
func (m *Model) needsTyping(i int) bool {
	u := &m.units[i]
	if !u.Typeable() {
		return false
	}
	if !m.prefs.SkipPunct {
		return true
	}
	for _, c := range u.Cells {
		if !c.Skip && markdown.IsAlnum(c.R) {
			return true
		}
	}
	return false
}

func (m *Model) countTotal() {
	m.total = 0
	for i := range m.units {
		if m.needsTyping(i) {
			m.total++
		}
	}
}

func (m *Model) match(typed, want rune) bool {
	return markdown.Matches(typed, want, m.prefs.CaseSensitive)
}

// Init has Claude write the questions of a file read actively.
func (m *Model) Init() tea.Cmd { return m.startGen() }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// A key that types past the end of a section opens its checkpoint.
	_, isKey := msg.(tea.KeyMsg)
	typing := isKey && !m.Finished && m.ask == nil && m.pick == nil && !m.showPref && !m.showKeys
	before := m.cur
	_, cmd := m.update(msg)
	if typing {
		cmd = tea.Batch(cmd, m.checkpoint(before))
	}
	// The last checkpoint comes before going back to the folder.
	if m.Folder && m.Finished && !m.closed && !m.checking() {
		m.closed = true
		m.leave()
		return m, closeDoc(true)
	}
	return m, cmd
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.cache = map[int]cached{}
	case askDeltaMsg, askDoneMsg:
		return m, m.askEvent(msg)
	case quizTickMsg:
		return m, m.armEvent(msg)
	case tea.KeyMsg:
		m.notice = ""
		// The key list closes on any key.
		if m.showKeys {
			m.showKeys = false
			return m, nil
		}
		// Terminals send ctrl+/ as ctrl+_.
		if msg.Type == tea.KeyCtrlUnderscore && !m.showPref && (!m.Finished || m.checking()) {
			m.showKeys = true
			return m, nil
		}
		if m.pick != nil {
			m.updatePicker(msg.String())
			return m, nil
		}
		if m.quiz != nil {
			return m, m.updateQuiz(msg)
		}
		if m.ask != nil {
			return m, m.updateAsk(msg)
		}
		if msg.Type == tea.KeyCtrlP && !m.Finished {
			m.showPref = !m.showPref
			return m, nil
		}
		if m.showPref {
			m.updatePrefs(msg)
			return m, nil
		}
		if m.Finished {
			switch msg.String() {
			case "r":
				m.cur = -1
				m.Finished = false
				m.next()
			case "q", "esc", "ctrl+c", "enter":
				m.leave()
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "up":
			m.scroll--
			return m, nil
		case "down":
			m.scroll++
			return m, nil
		case "pgup":
			m.scroll -= m.h / 2
			return m, nil
		case "pgdown":
			m.scroll += m.h / 2
			return m, nil
		}
		// Any other key brings the cursor back in view. Esc does only that.
		if m.scroll != 0 {
			m.scroll = 0
			if msg.Type == tea.KeyEsc {
				return m, nil
			}
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.leave()
			if m.Folder && msg.Type == tea.KeyEsc {
				return m, closeDoc(false)
			}
			return m, tea.Quit
		case tea.KeyCtrlO:
			if m.Folder {
				m.leave()
				return m, closeDoc(false)
			}
		case tea.KeyCtrlA:
			m.withModel(m.openAsk)
		case tea.KeyCtrlE:
			m.withModel(m.openExplain)
		case tea.KeyCtrlG:
			return m, m.quizNow()
		case tea.KeyCtrlT:
			return m, m.toggleActive()
		case tea.KeyTab:
			m.widen(1)
		case tea.KeyShiftTab:
			m.widen(-1)
		case tea.KeyCtrlL:
			switch m.sel {
			case scopeSection:
				m.nextSection()
			case scopeBlock:
				m.next()
			default:
				m.nextSentence()
			}
		case tea.KeyCtrlH:
			switch m.sel {
			case scopeSection:
				m.prevSection()
			case scopeBlock:
				m.prevBlock()
			default:
				m.prevSentence()
			}
		case tea.KeyCtrlJ:
			m.nextLine()
		case tea.KeyCtrlK:
			m.prevLine()
		case tea.KeyBackspace:
			if msg.Alt {
				m.deleteWord()
			} else {
				m.backspace()
			}
		case tea.KeyCtrlW:
			m.deleteWord()
		case tea.KeySpace:
			m.tick()
			m.typeRune(' ')
		case tea.KeyEnter:
			if msg.Alt {
				m.nextLine()
			} else {
				m.tick()
				m.enter()
			}
		case tea.KeyRunes:
			if !msg.Paste {
				m.tick()
				for _, r := range msg.Runes {
					m.typeRune(r)
				}
			}
		}
	}
	return m, nil
}

func (m *Model) updatePrefs(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc", "q", "ctrl+c":
		m.showPref = false
	case "up", "k", "shift+tab":
		m.prefSel = (m.prefSel + prefRows - 1) % prefRows
	case "down", "j", "tab":
		m.prefSel = (m.prefSel + 1) % prefRows
	case " ", "enter", "x":
		if m.prefSel == modelRow {
			m.pick = newPicker(m.prefs.Model, false)
			return
		}
		f := prefItems[m.prefSel].field(&m.prefs)
		*f = !*f
		m.prefErr = m.prefs.Save()
		m.countTotal()
		if m.complete() {
			m.next()
		}
	}
}

func (m *Model) unit() *markdown.Unit { return &m.units[m.cur] }

// withModel runs f once a model is chosen, asking for one the first time.
func (m *Model) withModel(f func()) {
	if m.prefs.Model != "" {
		f()
		return
	}
	m.pick = newPicker("", true)
	m.pick.then = f
}

// widen moves the selection one step wider, or narrower with d = -1, around
// from the section to the sentence. A block of one sentence has no step
// between the two.
func (m *Model) widen(d int) {
	n := scope((int(m.sel) + d + 3) % 3)
	if m.curSentence() == "" && (n == scopeBlock && d > 0 || n == scopeSentence && d < 0) {
		n = scope((int(n) + d + 3) % 3)
	}
	m.sel = n
}

// firstTyping is the first unit to type from unit i on, or -1.
func (m *Model) firstTyping(i int) int {
	for ; i < len(m.units); i++ {
		if m.needsTyping(i) {
			return i
		}
	}
	return -1
}

// nextSection skips the rest of the section the cursor is in.
func (m *Model) nextSection() {
	m.cur = m.secs[sectionOf(m.secs, m.cur)].end - 1
	m.next()
}

// prevSection goes back to the start of the section the cursor is in.
// Already there, it goes to the start of the section before.
func (m *Model) prevSection() {
	s := sectionOf(m.secs, m.cur)
	first := m.firstTyping(m.secs[s].start)
	if first == m.cur && !m.typedSince(0) && s > 0 {
		first = m.firstTyping(m.secs[s-1].start)
	}
	if first < 0 {
		return
	}
	m.cur = first
	m.typed = m.typed[:0]
	m.autoSkip()
}

// toCheckpoint is the words left to type before the section ends.
func (m *Model) toCheckpoint() int {
	n, in := 0, false
	for _, c := range m.unit().Cells[len(m.typed):] {
		w := !c.Skip && markdown.IsAlnum(c.R)
		if w && !in {
			n++
		}
		in = w
	}
	for i := m.cur + 1; i < m.secs[sectionOf(m.secs, m.cur)].end; i++ {
		if m.needsTyping(i) {
			n += markdown.Words(&m.units[i])
		}
	}
	return n
}

// next moves to the following typeable unit, or finishes the document.
func (m *Model) next() {
	m.typed = m.typed[:0]
	for m.cur++; m.cur < len(m.units); m.cur++ {
		if m.needsTyping(m.cur) {
			m.autoSkip()
			store.SaveProgress(m.Path, m.cur, len(m.units))
			return
		}
	}
	m.Finished = true
	store.MarkDone(m.Path, len(m.units))
}

func (m *Model) prev() {
	for i := m.cur - 1; i >= 0; i-- {
		if m.needsTyping(i) {
			m.cur = i
			m.typed = m.typed[:0]
			m.autoSkip()
			return
		}
	}
	m.typed = m.typed[:0]
	m.autoSkip()
}

// autoSkip fills in cells you are not asked to type.
func (m *Model) autoSkip() {
	cs := m.unit().Cells
	for len(m.typed) < len(cs) && cs[len(m.typed)].Skip {
		m.typed = append(m.typed, cs[len(m.typed)].R)
	}
}

func (m *Model) typeRune(r rune) {
	cs := m.unit().Cells
	p := len(m.typed)
	if p >= len(cs) {
		return
	}
	k := p
	if !m.match(r, cs[p].R) {
		k = -1
		if m.prefs.SkipPunct {
			// Look past punctuation and spaces for the key just pressed.
			for i := p; i < len(cs) && k < 0; i++ {
				if m.match(r, cs[i].R) {
					k = i
				} else if markdown.IsAlnum(cs[i].R) {
					break
				}
			}
		}
		if k < 0 {
			k = p // a wrong key is recorded as a mistake
		}
	}
	for ; p < k; p++ {
		m.typed = append(m.typed, cs[p].R)
	}
	m.typed = append(m.typed, r)
	if markdown.IsAlnum(cs[p].R) && (p == 0 || !markdown.IsAlnum(cs[p-1].R)) {
		m.words++
		m.wordAt = append(m.wordAt, m.typedSecs)
		if n := len(m.wordAt); n > burstWords {
			// Faster than anyone types is key repeat or a script.
			if dt := m.typedSecs - m.wordAt[n-1-burstWords]; dt > 0 && burstWords/dt*60 <= maxWPM {
				m.sess.BestWPM = max(m.sess.BestWPM, burstWords/dt*60)
			}
		}
	}
	m.autoSkip()
	if m.complete() {
		m.next()
	}
}

// enter acts as a space, so the end of a wrapped line can be typed with it.
func (m *Model) enter() {
	cs := m.unit().Cells
	for k := len(m.typed); k < len(cs); k++ {
		if cs[k].R == ' ' {
			m.typeRune(' ')
			return
		}
		if !m.prefs.SkipPunct || markdown.IsAlnum(cs[k].R) {
			return
		}
	}
}

// nextLine fills in the rest of the screen line the cursor is on, so typing
// carries on at the start of the next one. On the last line it ends the block.
func (m *Model) nextLine() {
	cs := m.unit().Cells
	end := len(cs)
	for _, rg := range view.LineRanges(m.unit(), m.contentWidth()) {
		if len(m.typed) < rg[1] {
			end = rg[1]
			break
		}
	}
	m.fillTo(end)
	if m.complete() {
		m.next()
	}
}

// prevLine un-types back to the start of the screen line the cursor is on.
// Already at a line start, it goes to the start of the previous line, which
// may be the last line of the previous block.
func (m *Model) prevLine() {
	rs := view.LineRanges(m.unit(), m.contentWidth())
	li := len(rs) - 1
	for i, rg := range rs {
		if len(m.typed) < rg[1] {
			li = i
			break
		}
	}
	switch {
	case m.typedSince(rs[li][0]):
		m.backTo(rs[li][0])
	case li > 0:
		m.backTo(rs[li-1][0])
	default:
		m.backBlock(func(u *markdown.Unit) int {
			rs := view.LineRanges(u, m.contentWidth())
			return rs[len(rs)-1][0]
		})
	}
}

// nextSentence fills in the rest of the sentence the cursor is in and the
// spaces after it. On the last sentence it ends the block. Sentences are the
// ones the underline shows (see markdown.Sentence).
func (m *Model) nextSentence() {
	cs := m.unit().Cells
	_, end := markdown.Sentence(cs, len(m.typed))
	for end < len(cs) && cs[end].R == ' ' {
		end++
	}
	m.fillTo(end)
	if m.complete() {
		m.next()
	}
}

// prevSentence un-types back to the start of the sentence the cursor is in.
// Already at a sentence start, it goes to the start of the one before, which
// may be the last sentence of the previous block.
func (m *Model) prevSentence() {
	cs := m.unit().Cells
	start, _ := markdown.Sentence(cs, len(m.typed))
	switch {
	case m.typedSince(start):
		m.backTo(start)
	case start > 0:
		m.backTo(sentenceBefore(cs, start))
	default:
		m.backBlock(func(u *markdown.Unit) int { return sentenceBefore(u.Cells, len(u.Cells)) })
	}
}

// prevBlock un-types the block the cursor is in. Already at its start, it
// goes to the previous block.
func (m *Model) prevBlock() {
	if m.typedSince(0) {
		m.backTo(0)
	} else {
		m.prev()
	}
}

// sentenceBefore is the start of the sentence that ends before cell i.
func sentenceBefore(cs []markdown.Cell, i int) int {
	k := i - 1
	for k > 0 && cs[k].R == ' ' {
		k--
	}
	if k <= 0 {
		return 0
	}
	// From the sentence's last cell, sentence looks back past its own end.
	start, _ := markdown.Sentence(cs, k)
	return start
}

// typedSince tells whether anything from cell i on was typed, rather than
// only filled in for you.
func (m *Model) typedSince(i int) bool {
	cs := m.unit().Cells
	for ; i < len(m.typed); i++ {
		if !cs[i].Skip {
			return true
		}
	}
	return false
}

// backTo un-types back to cell i.
func (m *Model) backTo(i int) {
	m.typed = m.typed[:min(i, len(m.typed))]
	m.autoSkip()
}

// backBlock moves to the previous block with everything before cell at(u)
// filled in. On the first block it stays put.
func (m *Model) backBlock(at func(u *markdown.Unit) int) {
	cur := m.cur
	m.prev()
	if m.cur != cur {
		m.fillTo(at(m.unit()))
	}
}

// fillTo types the right runes up to cell end.
func (m *Model) fillTo(end int) {
	cs := m.unit().Cells
	for len(m.typed) < end {
		m.typed = append(m.typed, cs[len(m.typed)].R)
	}
	m.autoSkip()
}

// complete is true once nothing that needs typing is left, mistakes or not.
// With skip punctuation on, trailing punctuation is free.
func (m *Model) complete() bool {
	cs := m.unit().Cells
	for _, c := range cs[len(m.typed):] {
		if !m.prefs.SkipPunct || markdown.IsAlnum(c.R) {
			return false
		}
	}
	return true
}

func (m *Model) backspace() bool {
	cs := m.unit().Cells
	k := len(m.typed)
	for k > 0 && cs[k-1].Skip {
		k--
	}
	if k == 0 {
		return false
	}
	m.typed = m.typed[:k-1]
	return true
}

func (m *Model) deleteWord() {
	cs := m.unit().Cells
	for len(m.typed) > 0 && cs[len(m.typed)-1].R == ' ' {
		if !m.backspace() {
			return
		}
	}
	for len(m.typed) > 0 && cs[len(m.typed)-1].R != ' ' {
		if !m.backspace() {
			return
		}
	}
}

func (m *Model) contentWidth() int { return min(m.w-4, maxWidth) }

func (m *Model) linesFor(i int) []string {
	done := i < m.cur
	if c, ok := m.cache[i]; ok && c.done == done {
		return c.lines
	}
	lines, _ := view.RenderUnit(&m.units[i], m.contentWidth(), nil, done)
	m.cache[i] = cached{done, lines}
	return lines
}

func (m *Model) View() string {
	if m.w == 0 {
		return ""
	}
	if m.pick != nil {
		return m.pickerView()
	}
	if m.showPref {
		return m.prefsView()
	}
	if m.showKeys {
		return m.keysView()
	}
	if m.quiz != nil {
		return m.quizView()
	}
	// A checkpoint takes the whole screen, also at the end of the file.
	if m.ask != nil && m.ask.review {
		return m.askView()
	}
	if m.Finished {
		return m.finishView()
	}
	if m.ask != nil && !m.prefs.SplitAsk {
		return m.askView()
	}
	cw := m.contentWidth()
	margin := strings.Repeat(" ", max((m.w-cw)/2, 0))
	docMargin := margin[min(len(gutter), len(margin)):] // docLines brings the gutter
	var pane []string
	// The footer wraps on a narrow screen, and the text gives it the room.
	foot := m.footer(cw)
	bodyH := max(m.h-3-len(foot), 1)
	if m.ask != nil {
		pane = m.askPane(cw, max(m.h*45/100, 6))
		bodyH = max(m.h-3-len(pane), 1) // header, blank and separator
	}
	body := m.docLines(cw, bodyH)

	var sb strings.Builder
	sb.WriteString(margin + m.header(cw) + "\n\n")
	for _, l := range body {
		sb.WriteString(docMargin + l + "\n")
	}
	if pane != nil {
		// The pane brings its own help line, in place of the blank and help below.
		sb.WriteString(margin + theme.Fg(theme.Overlay0).Render(rule(cw, "ask")) + "\n")
		for i, l := range pane {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(margin + l)
		}
		return sb.String()
	}
	for _, l := range foot {
		sb.WriteString("\n" + margin + l)
	}
	return sb.String()
}

// wrapHints lays footer entries out in lines of at most w columns, joined
// by dots. Entries are already styled, and never cut.
func wrapHints(w int, hs []string) []string {
	const sep = " · "
	var lines []string
	for _, h := range hs {
		if n := len(lines); n > 0 && lipgloss.Width(lines[n-1])+lipgloss.Width(sep)+lipgloss.Width(h) <= w {
			lines[n-1] += theme.Fg(theme.Overlay0).Render(sep) + h
		} else {
			lines = append(lines, h)
		}
	}
	return lines
}

// footer shows the keys that matter where the cursor is, in the width of the
// text, on as many lines as they need.
func (m *Model) footer(cw int) []string {
	dim := theme.Fg(theme.Overlay0)
	switch {
	case m.notice != "":
		return []string{dim.Render(runewidth.Truncate(m.notice, cw, "…"))}
	case m.scroll != 0:
		return []string{dim.Render(runewidth.Truncate("↑/↓ scroll · type or esc to return to the cursor", cw, "…"))}
	}
	// The selection comes first, the scope the underline covers standing out.
	names := make([]string, len(scopeNames))
	for i, n := range scopeNames {
		st := dim
		if scope(i) == m.sel {
			st = theme.Fg(theme.Subtext1).Underline(true)
		}
		names[i] = st.Render(n)
	}
	hs := []string{dim.Render("tab ") + strings.Join(names, " ")}
	rest := []string{"ctrl+l skip " + scopeNames[m.sel], "ctrl+a ask", "ctrl+e explain"}
	if m.check != nil {
		rest = append(rest, "ctrl+g quiz")
	} else {
		rest = append(rest, "ctrl+t active reading")
	}
	if m.Folder {
		rest = append(rest, "esc files")
	} else {
		rest = append(rest, "esc quit")
	}
	for _, h := range append(rest, view.KeysHint) {
		hs = append(hs, dim.Render(h))
	}
	return wrapHints(cw, hs)
}

// docLines is the document around the cursor, exactly bodyH lines tall.
func (m *Model) docLines(cw, bodyH int) []string {
	cur, curLine := view.RenderUnit(m.unit(), cw, &view.Cursor{Typed: m.typed, CaseSensitive: m.prefs.CaseSensitive, WholeBlock: m.sel != scopeSentence}, false)
	cur = m.withGutter(m.cur, cur)

	// Keep the cursor about a third of the way down the screen, then move the
	// view by the scroll offset.
	third := bodyH / 3
	var above []string
	for i := m.cur - 1; i >= 0 && len(above)+curLine < third+max(-m.scroll, 0); i-- {
		above = append(m.withGutter(i, m.linesFor(i)), above...)
	}
	lines := append(above, cur...)
	at := len(above) + curLine // the cursor line in lines
	base := max(at-third, 0)
	for i := m.cur + 1; i < len(m.units) && len(lines) < base+m.scroll+bodyH; i++ {
		lines = append(lines, m.withGutter(i, m.linesFor(i))...)
	}

	// Scrolling stops at the top of the file, and going down, once its end is
	// at the bottom of the screen.
	start := base + m.scroll
	if m.scroll > 0 {
		start = min(start, max(len(lines)-bodyH, base))
	}
	start = max(start, 0)
	m.scroll = start - base

	body := lines[start:min(start+bodyH, len(lines))]
	for len(body) < bodyH {
		body = append(body, "")
	}
	return body
}

// gutter is the space left of the text where a dot marks blocks that have a
// conversation, and a bar the section when it is the selection.
const gutter = "  "

func (m *Model) withGutter(i int, lines []string) []string {
	g := gutter
	s := sectionOf(m.secs, i)
	if m.sel == scopeSection && s == sectionOf(m.secs, m.cur) {
		g = theme.Fg(theme.Yellow).Render("▏") + " "
	}
	out := make([]string, len(lines))
	for k, l := range lines {
		out[k] = g + l
	}
	if m.asked[i] && len(out) > 0 {
		out[0] = theme.Fg(theme.Purple).Render("●") + " " + lines[0]
	}
	// The checkpoint of a section shows under its last block.
	if m.check != nil && i == m.secs[s].last(m.units) {
		out = append(out, "", gutter+m.checkMark(s))
	}
	return out
}

func (m *Model) header(cw int) string {
	doneN := 0
	for i := 0; i < m.cur; i++ {
		if m.needsTyping(i) {
			doneN++
		}
	}
	pct := 0
	if m.total > 0 {
		pct = doneN * 100 / m.total
	}
	const barW = 20
	filled := barW * pct / 100
	bar := theme.Fg(theme.Yellow).Render(strings.Repeat("━", filled)) +
		theme.Fg(theme.Overlay0).Render(strings.Repeat("━", barW-filled))
	right := bar + theme.Fg(theme.Subtext0).Render(fmt.Sprintf(" %3d%%", pct))
	left := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render(filepath.Base(m.Path))
	if m.check != nil {
		// The section the cursor is in. The checkpoint is only announced
		// once it is close.
		at := fmt.Sprintf("  ◆ %d/%d", sectionOf(m.secs, m.cur)+1, len(m.secs))
		more := theme.Fg(theme.Green).Render(at)
		if n := m.toCheckpoint(); n <= nearCheckpoint {
			soon := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Yellow)).Bold(true).Render(fmt.Sprintf("%s · checkpoint in %d", at, n))
			if lipgloss.Width(left)+lipgloss.Width(soon)+lipgloss.Width(right) < cw {
				more = soon
			}
		}
		// The questions being written show until they are there.
		if text, col := m.genStatus(); text != "" {
			st := theme.Fg(col).Render(" · " + text)
			if lipgloss.Width(left)+lipgloss.Width(more)+lipgloss.Width(st)+lipgloss.Width(right) < cw {
				more += st
			}
		}
		left += more
	}
	gap := max(cw-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) finishView() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Green)).Bold(true).Render("✔ done")
	name := theme.Fg(theme.Subtext1).Render(filepath.Base(m.Path))
	help := theme.Fg(theme.Overlay2).Render("r restart · q quit")
	rows := []string{title, "", name, ""}
	if s := sessionLine(m.tally()); s != "" {
		rows = append(rows, theme.Fg(theme.Subtext0).Render(s), "")
	}
	box := lipgloss.JoinVertical(lipgloss.Center, append(rows, help)...)
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, box)
}

// sessionLine sums a session up in one line, or "" when nothing was typed.
func sessionLine(t store.Tally) string {
	var parts []string
	if t.Words > 0 {
		parts = append(parts, fmt.Sprintf("%d words typed", t.Words))
	}
	if w := t.WPM(); w > 0 {
		parts = append(parts, fmt.Sprintf("%.0f wpm", w))
	}
	if t.BestWPM > 0 {
		parts = append(parts, fmt.Sprintf("top %.0f", t.BestWPM))
	}
	if t.Answered > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d questions right", t.Right, t.Answered))
	}
	return strings.Join(parts, " · ")
}
