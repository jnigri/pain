package reader

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	bcursor "github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"pain/internal/markdown"
	"pain/internal/store"
	"pain/internal/theme"
)

// Questions go to the claude CLI, so they run on whatever account it is
// logged into. --safe-mode keeps the login but skips the user's CLAUDE.md,
// skills and hooks, which would otherwise change how answers read.
var claudeArgs = []string{
	"-p", "--safe-mode", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
	"--include-partial-messages", "--tools", "", "--no-session-persistence",
}

const askPrompt = `You help someone who is reading a Markdown file in a terminal. They ask questions about one part of the file: a block, one sentence of it when a selection is given, or a whole section. The file's path, the headings the part sits under and the whole file are given for context, and earlier turns of the conversation come before the latest question.
Answer in Markdown, concisely: a few short paragraphs or a short list. Do not repeat the part back.`

type askMode int

const (
	askInput askMode = iota
	askStream
	askAnswer
	askHistory
)

// Limits on images, close to what the API accepts.
const (
	maxImages    = 5
	maxImageSize = 5 << 20
)

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

type askState struct {
	mode     askMode
	back     askMode // where esc goes from the history
	entry    store.AskEntry
	sentence string // the sentence a new conversation can be narrowed to, if any
	input    textarea.Model
	images   []string // attached to the question being typed
	pending  string   // question being answered
	pendImg  []string // its images
	partial  string   // its answer so far
	err      string
	scroll   int
	follow   bool // keep the end of the conversation in view
	anchor   int  // 1 + the turn to scroll to on the next draw, 0 for none
	cancel   context.CancelFunc
	ch       chan tea.Msg
	hist     []store.AskEntry // newest first
	sel      int
	// review makes this an explanation: the reader writes what revText says
	// in their own words and Claude checks it. Nothing of it is saved. sec is
	// the section whose checkpoint this is, or -1 when it was asked for.
	review   bool
	sec      int
	revTitle string
	revText  string
}

type askDeltaMsg struct {
	ch   chan tea.Msg
	text string
}

type askDoneMsg struct {
	ch     chan tea.Msg
	answer string
	err    error
}

// blockText is a unit as plain text, decoration included.
func blockText(u *markdown.Unit) string {
	rs := make([]rune, len(u.Cells))
	for i, c := range u.Cells {
		rs[i] = c.R
	}
	return u.Prefix + string(rs)
}

// askedUnit is the unit a conversation is about. It follows the block by its
// text if the file has changed since, and is -1 once the block is gone.
func (m *Model) askedUnit(e store.AskEntry) int {
	if e.Unit >= 0 && e.Unit < len(m.units) && blockText(&m.units[e.Unit]) == e.Block {
		return e.Unit
	}
	for i := range m.units {
		if blockText(&m.units[i]) == e.Block {
			return i
		}
	}
	return -1
}

// loadAsked marks the units that have a conversation, for the margin dots.
func (m *Model) loadAsked() {
	m.asked = map[int]bool{}
	for _, e := range store.ReadAsks()[m.Path] {
		if i := m.askedUnit(e); i >= 0 {
			m.asked[i] = true
		}
	}
}

// curSentence is the text of the sentence the cursor is in, or "" when it is
// the whole block. It ignores the underline mode.
func (m *Model) curSentence() string {
	cs := m.unit().Cells
	s, e := markdown.Sentence(cs, len(m.typed))
	if s == 0 && e == len(cs) {
		return ""
	}
	rs := make([]rune, 0, e-s)
	for _, c := range cs[s:e] {
		rs = append(rs, c.R)
	}
	return string(rs)
}

// openAsk asks about what the underline shows: the sentence the cursor is in,
// the whole block or the section. It reopens the latest conversation about
// it, if any.
func (m *Model) openAsk() {
	sent := m.curSentence()
	e := store.AskEntry{Unit: m.cur, Block: blockText(m.unit())}
	switch m.sel {
	case scopeSentence:
		e.Selection = sent
	case scopeSection:
		e.Section = true
	}
	sec := sectionOf(m.secs, m.cur)
	all := store.ReadAsks()[m.Path]
	for i := len(all) - 1; i >= 0; i-- {
		o := all[i]
		u := m.askedUnit(o)
		if u < 0 || o.Section != e.Section || o.Selection != e.Selection {
			continue
		}
		if u == m.cur || (o.Section && sectionOf(m.secs, u) == sec) {
			o.Unit = u
			m.ask = &askState{entry: o, sentence: sent, mode: askAnswer, anchor: len(o.Turns), input: newInput()}
			return
		}
	}
	m.ask = &askState{entry: e, sentence: sent, follow: true, input: newInput()}
}

// canWiden tells whether tab can still change what a conversation is about.
func (a *askState) canWiden() bool {
	return len(a.entry.Turns) == 0 && !a.review
}

// scope is what the conversation is about.
func (a *askState) scope() scope {
	switch {
	case a.entry.Section:
		return scopeSection
	case a.entry.Selection != "":
		return scopeSentence
	}
	return scopeBlock
}

// widen moves a new conversation from the sentence to the block, to the
// section, and around.
func (a *askState) widen() {
	switch a.scope() {
	case scopeSentence:
		a.entry.Selection = ""
	case scopeBlock:
		a.entry.Section = true
	default:
		a.entry.Section, a.entry.Selection = false, a.sentence
	}
}

// askTitle names the section a conversation is about.
func (m *Model) askTitle(e store.AskEntry) string {
	if i := m.askedUnit(e); i >= 0 {
		return "§ " + m.secs[sectionOf(m.secs, i)].title
	}
	return "§ " + e.Block
}

// section is the chain of headings unit i sits under, such as "Usage > Keys".
func (m *Model) section(i int) string {
	var heads []string
	level := 7
	if m.units[i].Kind == markdown.Heading {
		level = m.units[i].Level
	}
	for k := i - 1; k >= 0 && level > 1; k-- {
		u := &m.units[k]
		if u.Kind != markdown.Heading || u.Level >= level {
			continue
		}
		level = u.Level
		rs := make([]rune, len(u.Cells))
		for j, c := range u.Cells {
			rs[j] = c.R
		}
		heads = append([]string{strings.TrimSpace(string(rs))}, heads...)
	}
	return strings.Join(heads, " > ")
}

// selectionCells is the part of u's cells that reads sel, or nil if u does not
// hold it.
func selectionCells(u *markdown.Unit, sel string) []markdown.Cell {
	rs := make([]rune, len(u.Cells))
	for i, c := range u.Cells {
		rs[i] = c.R
	}
	idx := strings.Index(string(rs), sel)
	if idx < 0 {
		return nil
	}
	start := len([]rune(string(rs)[:idx]))
	return u.Cells[start : start+len([]rune(sel))]
}

// inputHeight is more rows than a question wraps to, so the textarea never
// scrolls and askPane shows only the rows in use.
const inputHeight = 200

// newInput is the question editor, with the usual shell keys for moving and
// deleting. The ask view keeps enter, ctrl+v and the arrows up and down.
func newInput() textarea.Model {
	t := textarea.New()
	t.ShowLineNumbers = false
	t.CharLimit, t.MaxHeight = 0, 0
	t.SetPromptFunc(2, func(i int) string {
		if i == 0 {
			return "› "
		}
		return ""
	})
	t.FocusedStyle = textarea.Style{
		Prompt: theme.Fg(theme.Yellow),
		Text:   theme.Fg(theme.Foreground),
	}
	t.FocusedStyle.CursorLine = t.FocusedStyle.Text
	t.Cursor.SetMode(bcursor.CursorStatic)
	t.Cursor.Style = theme.Fg(theme.Yellow)
	for _, b := range []*key.Binding{&t.KeyMap.InsertNewline, &t.KeyMap.Paste, &t.KeyMap.LineNext, &t.KeyMap.LinePrevious} {
		b.SetEnabled(false)
	}
	t.SetHeight(inputHeight)
	t.Focus()
	return t
}

// clipboardImage saves the image on the macOS clipboard to the cache and
// returns its path. Terminals only paste text, so pain reads it itself.
func clipboardImage() (string, error) {
	none := errors.New("no image in clipboard")
	out, err := exec.Command("osascript", "-e", "the clipboard as «class PNGf»").Output()
	s := strings.TrimSpace(string(out))
	if err != nil || !strings.HasPrefix(s, "«data PNGf") {
		return "", none
	}
	data, err := hex.DecodeString(strings.TrimSuffix(strings.TrimPrefix(s, "«data PNGf"), "»"))
	if err != nil || len(data) == 0 {
		return "", none
	}
	sum := sha256.Sum256(data)
	f := store.CachePath(filepath.Join("images", hex.EncodeToString(sum[:8])+".png"))
	if f == "" {
		return "", errors.New("no cache directory to keep the image in")
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(f, data, 0o644); err != nil {
		return "", err
	}
	return f, nil
}

// droppedImage is the image file named by pasted text, as when a file is
// dragged into the terminal, or "" when the text is not one.
func droppedImage(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	} else {
		// Shell escapes, such as "\ " for a space.
		var b strings.Builder
		for i := 0; i < len(s); i++ {
			if s[i] == '\\' && i+1 < len(s) {
				i++
			}
			b.WriteByte(s[i])
		}
		s = b.String()
	}
	if !imageExts[strings.ToLower(filepath.Ext(s))] {
		return ""
	}
	if st, err := os.Stat(s); err != nil || st.IsDir() {
		return ""
	}
	return s
}

func (a *askState) attach(path string) {
	st, err := os.Stat(path)
	switch {
	case len(a.images) >= maxImages:
		a.err = fmt.Sprintf("at most %d images per question", maxImages)
	case err != nil:
		a.err = err.Error()
	case st.Size() > maxImageSize:
		a.err = fmt.Sprintf("%s is over %d MB", filepath.Base(path), maxImageSize>>20)
	default:
		a.images, a.err = append(a.images, path), ""
	}
}

// userMessage is the question as a stream-json user message, its images
// first.
func userMessage(question string, images []string) ([]byte, error) {
	var content []map[string]any
	for _, p := range images {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		mt := http.DetectContentType(data)
		if !strings.HasPrefix(mt, "image/") {
			return nil, fmt.Errorf("%s is not an image", filepath.Base(p))
		}
		content = append(content, map[string]any{"type": "image", "source": map[string]any{
			"type": "base64", "media_type": mt, "data": base64.StdEncoding.EncodeToString(data),
		}})
	}
	if question != "" {
		content = append(content, map[string]any{"type": "text", "text": question})
	}
	return json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": content}})
}

// startAsk runs claude in the background. Its output arrives as askDeltaMsg
// values and one askDoneMsg on the returned channel, which is closed after.
// askArgs are the claude flags for one question. An empty or default model
// passes no --model, so Claude Code's own setting applies.
func askArgs(model, system string) []string {
	args := slices.Concat(claudeArgs, []string{"--system-prompt", system})
	if model != "" && model != store.ModelDefault {
		args = append(args, "--model", model)
	}
	return args
}

func startAsk(ctx context.Context, model, system, question string, images []string) chan tea.Msg {
	ch := make(chan tea.Msg)
	go func() {
		defer close(ch)
		send := func(msg tea.Msg) bool {
			select {
			case ch <- msg:
				return true
			case <-ctx.Done():
				return false
			}
		}
		input, err := userMessage(question, images)
		if err != nil {
			send(askDoneMsg{ch: ch, err: err})
			return
		}
		cmd := exec.CommandContext(ctx, "claude", askArgs(model, system)...)
		cmd.Dir = os.TempDir()
		cmd.Stdin = bytes.NewReader(append(input, '\n'))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.StdoutPipe()
		if err == nil {
			err = cmd.Start()
		}
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				err = errors.New("claude CLI not found: install Claude Code and log in")
			}
			send(askDoneMsg{ch: ch, err: err})
			return
		}

		var line struct {
			Type  string `json:"type"`
			Event struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			} `json:"event"`
			IsError bool   `json:"is_error"`
			Result  string `json:"result"`
		}
		var result *askDoneMsg
		sc := bufio.NewScanner(out)
		sc.Buffer(nil, 16<<20)
		for sc.Scan() {
			line.Event.Delta.Type = ""
			if json.Unmarshal(sc.Bytes(), &line) != nil {
				continue
			}
			switch {
			case line.Type == "stream_event" && line.Event.Delta.Type == "text_delta":
				if !send(askDeltaMsg{ch, line.Event.Delta.Text}) {
					_ = cmd.Wait()
					return
				}
			case line.Type == "result" && line.IsError:
				result = &askDoneMsg{ch: ch, err: errors.New(line.Result)}
			case line.Type == "result":
				result = &askDoneMsg{ch: ch, answer: line.Result}
			}
		}
		werr := cmd.Wait()
		if result == nil {
			err := werr
			if s := strings.TrimSpace(stderr.String()); s != "" {
				err = errors.New(s[strings.LastIndex(s, "\n")+1:])
			}
			if err == nil {
				err = errors.New("claude gave no answer")
			}
			result = &askDoneMsg{ch: ch, err: err}
		}
		send(*result)
	}()
	return ch
}

func waitAsk(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// send asks the question being typed. Each turn is a fresh claude run that
// gets the conversation so far as text, so no session is kept on disk.
func (m *Model) send() tea.Cmd {
	a := m.ask
	q := strings.TrimSpace(a.input.Value())
	if q == "" && len(a.images) == 0 {
		return nil
	}
	var system strings.Builder
	if a.review {
		system.WriteString(m.reviewSystem(a))
	} else {
		fmt.Fprintf(&system, "%s\n\nFile: %s\n", askPrompt, m.Path)
		i := m.askedUnit(a.entry)
		if i >= 0 {
			if s := m.section(i); s != "" {
				fmt.Fprintf(&system, "Section: %s\n", s)
			}
		}
		fmt.Fprintf(&system, "\n<file>\n%s\n</file>\n\n", m.src)
		if a.entry.Section && i >= 0 {
			fmt.Fprintf(&system, "The question is about this whole section of the file:\n<section>\n%s\n</section>",
				sectionText(m.units, m.secs[sectionOf(m.secs, i)]))
		} else {
			fmt.Fprintf(&system, "<block>\n%s\n</block>", a.entry.Block)
		}
		if a.entry.Selection != "" {
			fmt.Fprintf(&system, "\n\nThe question is about this sentence of the block:\n<selection>\n%s\n</selection>", a.entry.Selection)
		}
	}
	var prompt strings.Builder
	if len(a.entry.Turns) > 0 {
		prompt.WriteString("<conversation>\n")
		for _, t := range a.entry.Turns {
			// Images of earlier turns are not sent again, only named.
			fmt.Fprintf(&prompt, "<user>\n%s%s\n</user>\n<assistant>\n%s\n</assistant>\n",
				imageMarkers(t.Images), t.Question, t.Answer)
		}
		prompt.WriteString("</conversation>\n\n")
	}
	prompt.WriteString(q)
	ctx, cancel := context.WithCancel(context.Background())
	a.pending, a.partial = q, ""
	a.input.Reset()
	a.pendImg, a.images = a.images, nil
	a.mode, a.err, a.follow = askStream, "", true
	a.cancel, a.ch = cancel, startAsk(ctx, m.prefs.Model, system.String(), prompt.String(), a.pendImg)
	return waitAsk(a.ch)
}

func imageMarkers(images []string) string {
	var sb strings.Builder
	for _, p := range images {
		fmt.Fprintf(&sb, "[image: %s]\n", filepath.Base(p))
	}
	return sb.String()
}

// stop ends a running question and puts it back in the input, so it can be
// edited and sent again.
func (a *askState) stop() {
	a.cancel()
	a.ch, a.mode = nil, askInput
	a.input.SetValue(a.pending)
	a.images = a.pendImg
	a.pending, a.partial, a.pendImg = "", "", nil
}

func (m *Model) askEvent(msg tea.Msg) tea.Cmd {
	if cmd, ok := m.genEvent(msg); ok {
		return cmd
	}
	a := m.ask
	switch msg := msg.(type) {
	case askDeltaMsg:
		if a == nil || msg.ch != a.ch {
			return nil
		}
		a.partial += msg.text
		return waitAsk(a.ch)
	case askDoneMsg:
		if a == nil || msg.ch != a.ch {
			return nil
		}
		if msg.err != nil {
			a.stop()
			a.err = msg.err.Error()
			return nil
		}
		a.cancel()
		if a.review && len(a.entry.Turns) == 0 {
			m.sess.Explained++
			if m.check != nil && a.sec >= 0 {
				m.check.explained[a.sec] = true
			}
		}
		// Show the new answer from its start rather than its end.
		a.ch, a.mode, a.follow = nil, askAnswer, false
		a.anchor = len(a.entry.Turns) + 1
		if a.entry.Time.IsZero() {
			a.entry.Time = time.Now()
		}
		a.entry.Turns = append(a.entry.Turns, store.AskTurn{Question: a.pending, Answer: msg.answer, Images: a.pendImg})
		a.pending, a.partial, a.pendImg = "", "", nil
		if !a.review {
			store.SaveAsk(m.Path, a.entry)
			m.asked[a.entry.Unit] = true
		}
	}
	return nil
}

func (m *Model) updateAsk(msg tea.KeyMsg) tea.Cmd {
	a := m.ask
	key := msg.String()
	if key == "ctrl+r" && a.mode != askStream && a.mode != askHistory && !a.review {
		all := store.ReadAsks()[m.Path]
		a.hist = make([]store.AskEntry, len(all))
		for i, e := range all {
			a.hist[len(all)-1-i] = e
		}
		a.back, a.mode, a.sel = a.mode, askHistory, 0
		return nil
	}
	if a.mode != askHistory {
		// While typing, ctrl+u and ctrl+d edit the text instead.
		if a.mode != askInput {
			switch key {
			case "ctrl+u":
				key = "pgup"
			case "ctrl+d":
				key = "pgdown"
			}
		}
		switch key {
		case "up":
			a.scroll, a.follow = max(a.scroll-1, 0), false
			return nil
		case "down":
			a.scroll++
			return nil
		case "pgup":
			a.scroll, a.follow = max(a.scroll-m.h/2, 0), false
			return nil
		case "pgdown":
			a.scroll += m.h / 2
			return nil
		}
		// Scrolled up while typing or waiting, esc goes back to the end.
		if key == "esc" && !a.follow && a.mode != askAnswer {
			a.follow = true
			return nil
		}
	}
	switch a.mode {
	case askInput:
		if key != "ctrl+c" {
			a.follow = true // typing brings the input back in view
		}
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			if len(a.entry.Turns) == 0 {
				m.ask = nil
			} else {
				a.mode, a.err = askAnswer, ""
			}
		case tea.KeyEnter:
			return m.send()
		case tea.KeyTab:
			// On a checkpoint not answered yet, tab goes to the questions.
			if a.review {
				if len(a.entry.Turns) == 0 && a.sec >= 0 && m.check != nil {
					m.ask = nil
					m.setExplain(false)
					return m.openQuiz(a.sec)
				}
				break
			}
			// Same step as tab on the text, which follows it.
			if a.canWiden() {
				a.widen()
				if a.entry.Unit == m.cur {
					m.sel = a.scope()
				}
			}
		case tea.KeyCtrlV:
			if p, err := clipboardImage(); err != nil {
				a.err = err.Error()
			} else {
				a.attach(p)
			}
		default:
			empty := a.input.Value() == ""
			switch {
			// On an empty question, backspace removes the last image and
			// ctrl+u all of them.
			case empty && msg.Type == tea.KeyBackspace && !msg.Alt:
				if len(a.images) > 0 {
					a.images = a.images[:len(a.images)-1]
				}
			case empty && msg.Type == tea.KeyCtrlU:
				a.images = nil
			case msg.Type == tea.KeyRunes:
				// A file dragged into the terminal arrives as its pasted path.
				if p := droppedImage(string(msg.Runes)); p != "" && (msg.Paste || len(msg.Runes) > 1) {
					a.attach(p)
					break
				}
				// The question stays one line, so pasted newlines become spaces.
				msg.Runes = []rune(strings.ReplaceAll(string(msg.Runes), "\n", " "))
				a.input, _ = a.input.Update(msg)
			default:
				a.input, _ = a.input.Update(msg)
			}
		}
	case askStream:
		if key == "esc" || key == "ctrl+c" {
			a.stop()
		}
	case askAnswer:
		switch key {
		case "esc", "ctrl+c", "q":
			m.ask = nil
		case "enter", "r":
			a.mode, a.follow = askInput, true
		case "n":
			if a.review {
				break
			}
			a.entry = store.AskEntry{Unit: a.entry.Unit, Block: a.entry.Block, Selection: a.entry.Selection, Section: a.entry.Section}
			a.mode, a.scroll, a.follow = askInput, 0, true
			a.input.Reset()
		case "k":
			a.scroll, a.follow = max(a.scroll-1, 0), false
		case "j":
			a.scroll++
		case " ":
			a.scroll += m.h / 2
		case "g":
			if !a.review {
				m.jump(m.askedUnit(a.entry))
			}
		}
	case askHistory:
		switch key {
		case "esc", "ctrl+c":
			a.mode = a.back
		case "up", "k":
			a.sel = max(a.sel-1, 0)
		case "down", "j":
			a.sel = min(a.sel+1, max(len(a.hist)-1, 0))
		case "enter":
			if len(a.hist) > 0 {
				a.entry, a.mode, a.err, a.scroll, a.follow = a.hist[a.sel], askAnswer, "", 0, false
				a.sentence = a.entry.Selection
				if i := m.askedUnit(a.entry); i >= 0 {
					a.entry.Unit = i
				}
			}
		case "g":
			if len(a.hist) > 0 {
				m.jump(m.askedUnit(a.hist[a.sel]))
			}
		}
	}
	return nil
}

// jump closes the ask view and moves typing to unit i, or the next typeable
// unit after it when i is not typeable.
func (m *Model) jump(i int) {
	if i < 0 || i >= len(m.units) {
		return
	}
	m.ask = nil
	m.cur = i - 1
	m.next()
}
