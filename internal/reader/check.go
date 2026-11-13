package reader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pain/internal/store"
	"pain/internal/theme"
	"pain/internal/view"
)

// Active reading ends each section of a file with a checkpoint: questions
// Claude wrote about it when the file was first opened, or the section
// explained in your own words, which Claude then checks.

const quizPrompt = `You write quick multiple-choice questions for someone reading a document section by section. After each section, they answer your questions about it before moving on. The questions are not a test and must not try to catch the reader out: they keep the reading active, check that the section was read, and help its main ideas stick.

For each section, write 1 or 2 questions, 1 if the section is short. Ask about the main point of the section, the thing a reader should take away, never about details, exact wording, numbers or the layout of the document. Someone who read the section should get it right without hesitating.

Keep everything short and easy to read at a glance. The question is one short sentence. Each choice is a few words, at most about 8, never a full sentence when a phrase will do. Give 3 choices of similar length: exactly one is right, and the others are clearly wrong to anyone who read the section, with no trick wording, near-synonyms or half-right answers. Do not use "all of the above" or "none of the above". "why" is one short sentence recalling the point. Write plain text without Markdown, in the language of the document.

Reply with JSON only, one entry per section, in order:
{"sections":[{"questions":[{"q":"...","choices":["...","...","..."],"answer":0,"why":"..."}]}]}
"answer" is the index of the correct choice, counting from 0.`

const regenPrompt = `The reader already answered the questions below about this section and asked for new ones. Write different questions, about other points of the section or from another angle.`

const reviewPrompt = `You are a friendly study partner for someone reading a Markdown file. They just read one part of it, and they explain that part in their own words, from memory, to check that they understood it. The part can be a whole section, one block or a single sentence: expect an explanation of matching depth, and for a section they may not have read to its end, judge only what is in the part given.

Judge the substance only, and be generous. Loose wording, another order, their own examples and minor points left out are all fine. Do not nitpick.

- If they have the main ideas right, say so in one short sentence, such as "Yes, you got it.", and stop. No summary, no added detail, no list of what they could have said.
- If an important idea is missing, say in a few words that the rest is right, then point at the gap. Prefer one short question that leads them to it, such as "What happens when ...?", over giving the answer away. When the gap is a plain fact they cannot work out, just say it: "You missed that ...".
- If something is wrong, say which part, and give a hint or ask one question that helps them see why.

Keep it to a few sentences, and never ask more than one question at a time. When they answer or try again, tell them whether they have it now. If they are still stuck after that, or ask you directly, explain it plainly. They can also simply discuss the part: answer their own questions concisely.

Answer in Markdown, in the language they write in.`

// nearCheckpoint is the words left in a section under which the header
// announces the checkpoint.
const nearCheckpoint = 25

// checks is the active reading state of a file.
type checks struct {
	quiz      [][]store.Question // per section, nil until written
	score     []store.Score      // per section, how its questions went last
	hash      string             // of the file, to reuse saved questions
	passed    map[int]bool       // sections whose checkpoint was shown
	explained map[int]bool       // sections explained this session
	err       string             // why the questions could not be written
	cancel    context.CancelFunc
	ch        chan tea.Msg // set while Claude writes questions
	regen     int          // the section getting new questions, -1 for the whole file
}

// quizState is the checkpoint asking questions. qs is nil while the questions
// are being written.
type quizState struct {
	sec    int
	qs     []store.Question
	i      int // the question shown
	sel    int
	picked int // the choice made, -1 before
	right  int
	done   bool // every question answered: the score
}

// setupChecks turns checkpoints on for this file, reusing saved questions.
func (m *Model) setupChecks() {
	// The prompt is part of the hash, so questions written by an older
	// prompt are written again.
	sum := sha256.Sum256(append([]byte(quizPrompt), m.src...))
	c := &checks{hash: hex.EncodeToString(sum[:8]), passed: map[int]bool{}, explained: map[int]bool{}, regen: -1}
	c.quiz = store.LoadQuiz(m.Path, c.hash, len(m.secs))
	c.score = store.LoadScores(m.Path, c.hash, len(m.secs))
	m.check = c
}

// checking tells whether a checkpoint is on screen.
func (m *Model) checking() bool {
	return m.quiz != nil || (m.ask != nil && m.ask.review)
}

// toggleActive turns active reading on or off for the folder the file was
// opened from, or for the file alone.
func (m *Model) toggleActive() tea.Cmd {
	scope := m.Scope
	if scope == "" {
		scope = m.Path
	}
	on := m.prefs.ToggleActive(scope, m.Path)
	if err := m.prefs.Save(); err != nil {
		m.notice = "could not save: " + err.Error()
		return nil
	}
	if !on {
		m.stopGen()
		m.check = nil
		m.notice = "active reading off"
		return nil
	}
	m.setupChecks()
	m.notice = "active reading on for " + filepath.Base(scope) + " · a checkpoint ends each section"
	return m.startGen()
}

func sectionPrompt(m *Model, i int) string {
	s := m.secs[i]
	return fmt.Sprintf("<section number=\"%d\" title=%q>\n%s\n</section>\n\n", i+1, s.title, sectionText(m.units, s))
}

// startGen asks Claude for the questions of every section, once per file.
func (m *Model) startGen() tea.Cmd {
	c := m.check
	if c == nil || m.Finished || c.quiz != nil || c.ch != nil {
		return nil
	}
	var prompt strings.Builder
	for i := range m.secs {
		prompt.WriteString(sectionPrompt(m, i))
	}
	return m.gen(-1, prompt.String())
}

// startRegen asks Claude for other questions about one section.
func (m *Model) startRegen(sec int) tea.Cmd {
	c := m.check
	if c.quiz == nil || c.ch != nil {
		return nil
	}
	var prompt strings.Builder
	prompt.WriteString(sectionPrompt(m, sec) + regenPrompt + "\n")
	for _, q := range c.quiz[sec] {
		fmt.Fprintf(&prompt, "- %s\n", q.Q)
	}
	m.quiz = &quizState{sec: sec, picked: -1}
	return m.gen(sec, prompt.String())
}

func (m *Model) gen(regen int, prompt string) tea.Cmd {
	c := m.check
	ctx, cancel := context.WithCancel(context.Background())
	c.err, c.regen = "", regen
	c.cancel, c.ch = cancel, startAsk(ctx, m.prefs.Model, quizPrompt, prompt, nil)
	return waitAsk(c.ch)
}

func (m *Model) stopGen() {
	if c := m.check; c != nil && c.ch != nil {
		c.cancel()
		c.ch = nil
	}
}

// genEvent handles Claude's output when it is the questions being written.
func (m *Model) genEvent(msg tea.Msg) (tea.Cmd, bool) {
	c := m.check
	if c == nil || c.ch == nil {
		return nil, false
	}
	switch msg := msg.(type) {
	case askDeltaMsg:
		if msg.ch != c.ch {
			return nil, false
		}
		return waitAsk(c.ch), true
	case askDoneMsg:
		if msg.ch != c.ch {
			return nil, false
		}
		c.cancel()
		c.ch = nil
		err := msg.err
		switch {
		case err != nil:
		case c.regen < 0:
			c.quiz, err = parseQuiz(msg.answer, len(m.secs))
		default:
			var one [][]store.Question
			// The score stays until the new questions are answered.
			if one, err = parseQuiz(msg.answer, 1); err == nil {
				c.quiz[c.regen] = one[0]
			}
		}
		if err != nil {
			c.err = err.Error() // regen stays, so trying again asks for the same
			return nil, true
		}
		c.regen = -1
		store.SaveQuiz(m.Path, c.hash, c.quiz)
		if q := m.quiz; q != nil {
			m.quiz = nil
			return m.openQuiz(q.sec), true
		}
		return nil, true
	}
	return nil, false
}

// shuffle moves the choices of a question around, keeping track of the right
// one. Models tend to put it in the same place.
func shuffle(q *store.Question) {
	right := q.Choices[q.Answer]
	rand.Shuffle(len(q.Choices), func(a, b int) { q.Choices[a], q.Choices[b] = q.Choices[b], q.Choices[a] })
	q.Answer = slices.Index(q.Choices, right)
}

// parseQuiz reads Claude's questions for n sections. Questions that cannot
// be asked are dropped.
func parseQuiz(s string, n int) ([][]store.Question, error) {
	var out struct {
		Sections []struct {
			Questions []store.Question `json:"questions"`
		} `json:"sections"`
	}
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j < i || json.Unmarshal([]byte(s[i:j+1]), &out) != nil {
		return nil, errors.New("could not read the questions Claude wrote")
	}
	quiz := make([][]store.Question, n)
	found := false
	for k := 0; k < n && k < len(out.Sections); k++ {
		for _, q := range out.Sections[k].Questions {
			if q.Q == "" || len(q.Choices) < 2 || len(q.Choices) > 9 || q.Answer < 0 || q.Answer >= len(q.Choices) {
				continue
			}
			shuffle(&q)
			quiz[k] = append(quiz[k], q)
			found = true
		}
	}
	if !found {
		return nil, errors.New("Claude wrote no questions")
	}
	return quiz, nil
}

// checkpoint opens the checkpoint of the section just left, when a key moved
// typing from unit `before` into a later section or to the end of the file.
func (m *Model) checkpoint(before int) tea.Cmd {
	c := m.check
	if c == nil || before < 0 || before >= len(m.units) || (!m.Finished && m.cur <= before) {
		return nil
	}
	s := sectionOf(m.secs, before)
	if c.passed[s] || (!m.Finished && sectionOf(m.secs, m.cur) == s) {
		return nil
	}
	c.passed[s] = true
	if m.prefs.Explain {
		m.openReview(s)
		return nil
	}
	return m.openQuiz(s)
}

// quizNow opens the questions of the section the cursor is in, before its
// end or once more.
func (m *Model) quizNow() tea.Cmd {
	if m.check == nil {
		m.notice = "questions need active reading · ctrl+t turns it on"
		return nil
	}
	return m.openQuiz(sectionOf(m.secs, m.cur))
}

// openQuiz shows the questions of a section. A section Claude wrote none for
// is explained instead.
func (m *Model) openQuiz(sec int) tea.Cmd {
	c := m.check
	writing := c.ch != nil && c.regen == sec
	if c.quiz != nil && len(c.quiz[sec]) == 0 && !writing {
		m.openReview(sec)
		return nil
	}
	m.quiz = &quizState{sec: sec, picked: -1}
	switch {
	case writing:
	case c.quiz != nil:
		// Each try sees the choices in another order.
		for i := range c.quiz[sec] {
			shuffle(&c.quiz[sec][i])
		}
		m.quiz.qs, c.err = c.quiz[sec], ""
	case c.err == "": // else it is shown, with a key to try again
		return m.startGen()
	}
	return nil
}

// openReview asks for a section in your own words, as its checkpoint.
func (m *Model) openReview(sec int) {
	s := m.secs[sec]
	m.explain(sec, s.title, sectionText(m.units, s))
}

// openExplain asks for what is selected in your own words, at any time: the
// sentence, the block, or the section as far as it was read.
func (m *Model) openExplain() {
	sent := m.curSentence()
	switch {
	case m.sel == scopeSection:
		s := m.secs[sectionOf(m.secs, m.cur)]
		title := s.title
		if m.cur+1 < s.end {
			title += " · up to here"
		}
		m.explain(-1, title, sectionText(m.units, section{start: s.start, end: m.cur + 1}))
	case m.sel == scopeSentence && sent != "":
		m.explain(-1, "the sentence “"+snippet(sent)+"”", sent)
	default:
		text := blockText(m.unit())
		m.explain(-1, "the block “"+snippet(text)+"”", text)
	}
}

// explain opens the ask view on an explanation to write. sec is the section
// whose checkpoint this is, or -1.
func (m *Model) explain(sec int, title, text string) {
	m.ask = &askState{review: true, sec: sec, revTitle: title, revText: text,
		follow: true, input: newInput(), entry: store.AskEntry{Unit: -1}}
}

// snippet is the first words of s, enough to tell which text it is without
// giving it away.
func snippet(s string) string {
	words := strings.Fields(s)
	if len(words) <= 5 {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:5], " ") + "…"
}

// setExplain remembers the kind of checkpoint used last.
func (m *Model) setExplain(on bool) {
	m.prefs.Explain = on
	m.prefErr = m.prefs.Save()
}

// reviewSystem is the system prompt for checking an explanation.
func (m *Model) reviewSystem(a *askState) string {
	return fmt.Sprintf("%s\n\nFile: %s\nPart: %s\n\n<file>\n%s\n</file>\n\n<part>\n%s\n</part>",
		reviewPrompt, m.Path, a.revTitle, m.src, a.revText)
}

// checkMark is the line that shows where a section ends with a checkpoint,
// and how it went once passed.
func (m *Model) checkMark(sec int) string {
	c := m.check
	label, col := "◆ checkpoint", theme.Dim(theme.Green)
	sc := c.score[sec]
	switch {
	case sc.Total > 0 && sc.Right == sc.Total:
		label, col = fmt.Sprintf("✔ %d of %d right", sc.Right, sc.Total), theme.Green
	case sc.Total > 0:
		label, col = fmt.Sprintf("● %d of %d right", sc.Right, sc.Total), theme.Yellow
	case c.explained[sec]:
		label, col = "✔ explained", theme.Green
	case c.passed[sec]:
		label, col = "◇ checkpoint skipped", theme.Overlay2
	case c.quiz != nil && len(c.quiz[sec]) == 1:
		label += " · 1 question"
	case c.quiz != nil && len(c.quiz[sec]) > 1:
		label += fmt.Sprintf(" · %d questions", len(c.quiz[sec]))
	}
	return theme.Fg(col).Render(rule(m.contentWidth(), label))
}

func (m *Model) updateQuiz(msg tea.KeyMsg) tea.Cmd {
	q, c := m.quiz, m.check
	key := msg.String()
	next := key == "enter" || key == " "
	switch {
	case key == "esc" || key == "ctrl+c":
		m.quiz = nil
	case key == "tab" || key == "ctrl+e":
		m.quiz = nil
		m.setExplain(true)
		m.openReview(q.sec)
	case q.qs == nil:
		if key == "r" && c.err != "" {
			if c.regen >= 0 {
				return m.startRegen(c.regen)
			}
			return m.startGen()
		}
	case key == "ctrl+r":
		return m.startRegen(q.sec)
	case q.done:
		switch {
		case next || key == "q":
			m.quiz = nil
		case key == "r":
			return m.openQuiz(q.sec)
		}
	case q.picked >= 0:
		if next {
			q.i, q.sel, q.picked = q.i+1, 0, -1
			if q.done = q.i >= len(q.qs); q.done {
				c.score[q.sec] = store.Score{Right: q.right, Total: len(q.qs)}
				store.SaveScores(m.Path, c.hash, c.score)
			}
		}
	default:
		n := len(q.qs[q.i].Choices)
		pick := -1
		switch {
		case key == "up" || key == "k":
			q.sel = (q.sel + n - 1) % n
		case key == "down" || key == "j":
			q.sel = (q.sel + 1) % n
		case next:
			pick = q.sel
		case len(key) == 1 && key[0] >= '1' && int(key[0]-'1') < n:
			pick = int(key[0] - '1')
		}
		if pick >= 0 {
			q.sel, q.picked = pick, pick
			m.sess.Answered++
			if pick == q.qs[q.i].Answer {
				q.right++
				m.sess.Right++
			}
		}
	}
	return nil
}

// wrapped is s in style st, wrapped to w, with the first line after `first`
// and the others indented to match.
func wrapped(st lipgloss.Style, s string, w int, first string) []string {
	ind := lipgloss.Width(first)
	lines := strings.Split(st.Width(max(w-ind, 10)).Render(s), "\n")
	for i, l := range lines {
		if i == 0 {
			lines[i] = first + l
		} else {
			lines[i] = strings.Repeat(" ", ind) + l
		}
	}
	return lines
}

func (m *Model) quizView() string {
	q, c := m.quiz, m.check
	cw := m.contentWidth()
	margin := strings.Repeat(" ", max((m.w-cw)/2, 0))
	bold := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Foreground)).Bold(true)

	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render("checkpoint · questions")
	name := theme.Fg(theme.Subtext0).Render(fmt.Sprintf("%s · section %d of %d", filepath.Base(m.Path), q.sec+1, len(m.secs)))
	gap := max(cw-lipgloss.Width(title)-lipgloss.Width(name), 1)
	lines := []string{title + strings.Repeat(" ", gap) + name, ""}
	lines = append(lines, wrapped(theme.Fg(theme.Subtext0), m.secs[q.sec].title, cw, "")...)
	lines = append(lines, "")

	var help string
	switch {
	case q.qs == nil && c.err != "":
		lines = append(lines, wrapped(theme.Fg(theme.Red), c.err, cw, "")...)
		help = "r try again · ctrl+e explain it instead · esc skip"
	case q.qs == nil:
		lines = append(lines, theme.Fg(theme.Overlay2).Render("writing the questions…"))
		help = "ctrl+e explain it instead · esc skip"
	case q.done:
		col, mark := theme.Green, "✔"
		if q.right < len(q.qs) {
			col, mark = theme.Yellow, "●"
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Bold(true).
			Render(fmt.Sprintf("%s %d of %d right", mark, q.right, len(q.qs))))
		help = "enter carry on · r answer again · ctrl+r new questions · ctrl+e explain it too"
	default:
		cur := q.qs[q.i]
		lines = append(lines, theme.Fg(theme.Overlay2).Render(fmt.Sprintf("question %d of %d", q.i+1, len(q.qs))), "")
		lines = append(lines, wrapped(bold, cur.Q, cw, "")...)
		lines = append(lines, "")
		for i, ch := range cur.Choices {
			mark, st := "  ", theme.Fg(theme.Subtext1)
			switch {
			case q.picked < 0 && i == q.sel:
				mark, st = theme.Fg(theme.Yellow).Render("› "), bold
			case q.picked >= 0 && i == cur.Answer:
				mark, st = theme.Fg(theme.Green).Render("✔ "), theme.Fg(theme.Green)
			case q.picked >= 0 && i == q.picked:
				mark, st = theme.Fg(theme.Red).Render("✘ "), theme.Fg(theme.Red)
			case q.picked >= 0:
				st = theme.Fg(theme.Overlay2)
			}
			lines = append(lines, wrapped(st, ch, cw, mark+theme.Fg(theme.Overlay2).Render(fmt.Sprintf("%d  ", i+1)))...)
		}
		help = "↑/↓ move · enter or 1–" + fmt.Sprint(len(cur.Choices)) + " answer · ctrl+e explain it instead · esc skip"
		if q.picked >= 0 {
			verdict := theme.Fg(theme.Green).Render("Right.")
			if q.picked != cur.Answer {
				verdict = theme.Fg(theme.Red).Render("Not quite.")
			}
			lines = append(lines, "", verdict)
			if cur.Why != "" {
				lines = append(lines, wrapped(theme.Fg(theme.Subtext1), cur.Why, cw, "")...)
			}
			help = "enter next question · esc skip"
			if q.i == len(q.qs)-1 {
				help = "enter see the score · esc skip"
			}
		}
	}

	// The help line stays at the bottom of the screen.
	bodyH := max(m.h-2, 1)
	if len(lines) > bodyH {
		lines = lines[:bodyH]
	}
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	lines = append(lines, "", theme.Fg(theme.Overlay0).Render(help+" · "+view.KeysHint))
	return margin + strings.Join(lines, "\n"+margin)
}
