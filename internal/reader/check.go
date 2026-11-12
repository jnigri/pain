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

const quizPrompt = `You write multiple-choice questions that check whether a reader understood a document they are reading section by section. After each section, the reader answers your questions about it before moving on.

For each section, write 2 or 3 questions, or 1 if the section is very short. Ask about the ideas that matter: what the section claims, why, how things relate, what follows from them. Do not ask about trivia, exact wording, numbers nobody would remember, or the layout of the document. A reader who understood the section should find the questions easy, and one who skimmed it should not be able to guess.

Each question has 4 choices of similar length and style. Exactly one is correct; the others are plausible but clearly wrong to someone who understood. Do not use "all of the above" or "none of the above". "why" is one short sentence saying why the correct choice is right. Write plain text without Markdown, in the language of the document.

Reply with JSON only, one entry per section, in order:
{"sections":[{"questions":[{"q":"...","choices":["...","...","...","..."],"answer":0,"why":"..."}]}]}
"answer" is the index of the correct choice, counting from 0.`

const reviewPrompt = `You check that someone reading a Markdown file understood the section they just finished. They explain the section in their own words, from memory, and you tell them whether they got it.

Judge the substance only. If they have the main ideas right, say so in one short sentence, such as "You got it.", and stop: no summary, no praise, no extra detail. Do not nitpick wording, loose phrasing or minor points they left out. Only when something important is wrong or missing, say what, in a few sentences at most, so they can fix their understanding.

If they then ask questions or try again, answer concisely. Answer in Markdown, in the language they write in.`

// checks is the active reading state of a file.
type checks struct {
	secs   []section
	quiz   [][]store.Question // per section, nil until written
	hash   string             // of the file, to reuse saved questions
	passed map[int]bool       // sections whose checkpoint was shown
	err    string             // why the questions could not be written
	cancel context.CancelFunc
	ch     chan tea.Msg // set while Claude writes the questions
}

// quizState is the checkpoint asking questions. It waits while the questions
// are still being written.
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
	sum := sha256.Sum256(m.src)
	c := &checks{secs: splitSections(m.units), hash: hex.EncodeToString(sum[:8]), passed: map[int]bool{}}
	c.quiz = store.LoadQuiz(m.Path, c.hash, len(c.secs))
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

// startGen asks Claude for the questions of every section, once per file.
func (m *Model) startGen() tea.Cmd {
	c := m.check
	if c == nil || m.Finished || c.quiz != nil || c.ch != nil {
		return nil
	}
	var prompt strings.Builder
	for i, s := range c.secs {
		fmt.Fprintf(&prompt, "<section number=\"%d\" title=%q>\n%s\n</section>\n\n", i+1, s.title, sectionText(m.units, s))
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.err = ""
	c.cancel, c.ch = cancel, startAsk(ctx, m.prefs.Model, quizPrompt, prompt.String(), nil)
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
		if err == nil {
			c.quiz, err = parseQuiz(msg.answer, len(c.secs))
		}
		if err != nil {
			c.err = err.Error()
			return nil, true
		}
		store.SaveQuiz(m.Path, c.hash, c.quiz)
		if q := m.quiz; q != nil {
			m.quiz = nil
			return m.openQuiz(q.sec), true
		}
		return nil, true
	}
	return nil, false
}

// parseQuiz reads Claude's questions for n sections. Questions that cannot
// be asked are dropped, and the choices are shuffled, as models tend to put
// the right one in the same place.
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
			right := q.Choices[q.Answer]
			rand.Shuffle(len(q.Choices), func(a, b int) { q.Choices[a], q.Choices[b] = q.Choices[b], q.Choices[a] })
			for a, c := range q.Choices {
				if c == right {
					q.Answer = a
				}
			}
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
	s := sectionOf(c.secs, before)
	if c.passed[s] || (!m.Finished && sectionOf(c.secs, m.cur) == s) {
		return nil
	}
	c.passed[s] = true
	if m.prefs.Explain {
		m.openReview(s)
		return nil
	}
	return m.openQuiz(s)
}

// openQuiz shows the questions of a section. A section Claude wrote none for
// is explained instead.
func (m *Model) openQuiz(sec int) tea.Cmd {
	c := m.check
	if c.quiz != nil && len(c.quiz[sec]) == 0 {
		m.openReview(sec)
		return nil
	}
	m.quiz = &quizState{sec: sec, picked: -1}
	if c.quiz != nil {
		m.quiz.qs = c.quiz[sec]
		return nil
	}
	if c.err != "" {
		return nil // shown, with a key to try again
	}
	return m.startGen()
}

// openReview asks for the section in your own words, in the ask view.
func (m *Model) openReview(sec int) {
	m.ask = &askState{review: true, sec: sec, follow: true, input: newInput(), entry: store.AskEntry{Unit: -1}}
}

// setExplain remembers the kind of checkpoint used last.
func (m *Model) setExplain(on bool) {
	m.prefs.Explain = on
	m.prefErr = m.prefs.Save()
}

// reviewSystem is the system prompt for checking an explanation of a section.
func (m *Model) reviewSystem(sec int) string {
	s := m.check.secs[sec]
	return fmt.Sprintf("%s\n\nFile: %s\nSection: %s\n\n<file>\n%s\n</file>\n\n<section>\n%s\n</section>",
		reviewPrompt, m.Path, s.title, m.src, sectionText(m.units, s))
}

func (m *Model) updateQuiz(msg tea.KeyMsg) tea.Cmd {
	q, c := m.quiz, m.check
	key := msg.String()
	next := key == "enter" || key == " "
	switch {
	case key == "esc" || key == "ctrl+c":
		m.quiz = nil
	case key == "tab":
		m.quiz = nil
		m.setExplain(true)
		m.openReview(q.sec)
	case c.quiz == nil:
		if key == "r" && c.err != "" {
			return m.startGen()
		}
	case q.done:
		if next || key == "q" {
			m.quiz = nil
		}
	case q.picked >= 0:
		if next {
			q.i, q.sel, q.picked = q.i+1, 0, -1
			q.done = q.i >= len(q.qs)
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
			if pick == q.qs[q.i].Answer {
				q.right++
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

	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Purple)).Bold(true).Render("checkpoint")
	name := theme.Fg(theme.Subtext0).Render(filepath.Base(m.Path))
	gap := max(cw-lipgloss.Width(title)-lipgloss.Width(name), 1)
	lines := []string{title + strings.Repeat(" ", gap) + name, ""}
	lines = append(lines, wrapped(theme.Fg(theme.Subtext0), c.secs[q.sec].title, cw, "")...)
	lines = append(lines, "")

	var help string
	switch {
	case c.quiz == nil && c.err != "":
		lines = append(lines, wrapped(theme.Fg(theme.Red), c.err, cw, "")...)
		help = "r try again · tab explain it instead · esc skip"
	case c.quiz == nil:
		lines = append(lines, theme.Fg(theme.Overlay2).Render("writing the questions…"))
		help = "tab explain it instead · esc skip"
	case q.done:
		col, mark := theme.Green, "✔"
		if q.right < len(q.qs) {
			col, mark = theme.Yellow, "●"
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Bold(true).
			Render(fmt.Sprintf("%s %d of %d right", mark, q.right, len(q.qs))))
		help = "enter carry on · tab explain it too"
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
		help = "↑/↓ move · enter or 1–" + fmt.Sprint(len(cur.Choices)) + " answer · tab explain it instead · esc skip"
		if q.picked >= 0 {
			verdict := theme.Fg(theme.Green).Render("Right.")
			if q.picked != cur.Answer {
				verdict = theme.Fg(theme.Red).Render("Not quite.")
			}
			lines = append(lines, "", verdict)
			if cur.Why != "" {
				lines = append(lines, wrapped(theme.Fg(theme.Subtext1), cur.Why, cw, "")...)
			}
			help = "enter next · esc skip"
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
