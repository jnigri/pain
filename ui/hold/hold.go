// Package hold is a hold-to-confirm control for Bubble Tea: a key must be
// held down for a while for its action to count, so a key hit by mistake
// does nothing. Underline draws how far along the hold is.
//
// Terminals do not tell a program when a key is let go. A held key is seen
// through its auto-repeat instead: the press, a pause, then the same key
// again and again. Hold learns the pause and the pace of the repeats as it
// sees them, so it notices a key let go about as soon as the next repeat
// fails to come. A hold only completes once at least one repeat came, so a
// single press never confirms.
package hold

import (
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// frame is how often a running hold ticks, for a smooth underline.
	frame = time.Second / 60
	// firstWait is the longest wait for the first repeat before the pause
	// was ever seen. A late repeat still resumes the hold, see lateWait.
	firstWait = 700 * time.Millisecond
	// lateWait is how long after a press its first repeat may still come
	// and take the hold up again.
	lateWait = 2 * time.Second
	// minGap and maxGap bound the wait for the next repeat once they come.
	minGap = 100 * time.Millisecond
	maxGap = 400 * time.Millisecond
)

var lastID atomic.Int64

// now is the clock, replaced in tests.
var now = time.Now

// TickMsg moves a running hold. Pass every message to Update, which keeps
// only the ticks of its own hold.
type TickMsg struct {
	id, tag int64
}

// Hold follows one key held down at a time. The zero value is not ready:
// make one with New, and keep it across holds, so what it learns of the
// keyboard's repeat carries over.
type Hold struct {
	// Duration is how long the key must be held.
	Duration time.Duration

	id, tag int64 // tag tells this hold's ticks from earlier ones
	key     string
	start   time.Time
	last    time.Time // when the key last came
	repeats int
	running bool
	lapsed  bool // gave up waiting for a first repeat, which may still come late

	// spent is the key that completed, ignored while it is still held so its
	// repeats do not go on to do something else.
	spent string

	delay time.Duration // seen from press to first repeat, 0 before
	every time.Duration // seen between repeats, 0 before
}

// New is a hold that completes after d.
func New(d time.Duration) Hold {
	return Hold{Duration: d, id: lastID.Add(1)}
}

// Start begins a hold of key, dropping any other one. Its returned command
// starts the ticks.
func (h *Hold) Start(key string) tea.Cmd {
	t := now()
	h.key, h.start, h.last = key, t, t
	h.repeats, h.running, h.lapsed, h.spent = 0, true, false, ""
	h.tag++
	return h.tick()
}

// Cancel drops the running hold.
func (h *Hold) Cancel() {
	h.running, h.lapsed = false, false
	h.tag++
}

// Running tells whether a key is being held.
func (h *Hold) Running() bool { return h.running }

// Key is the key being held, or that was last.
func (h *Hold) Key() string { return h.key }

// Progress is how far along the running hold is, from 0 to 1; 0 when none
// is running.
func (h *Hold) Progress() float64 {
	if !h.running || h.Duration <= 0 {
		return 0
	}
	return min(float64(now().Sub(h.start))/float64(h.Duration), 1)
}

// Repeat takes in a key that came and tells whether it is the auto-repeat of
// the key held, or of the one that completed. Such a key is the hold's: the
// caller ignores it. The command restarts the ticks of a hold taken up again
// by a late repeat.
func (h *Hold) Repeat(key string) (bool, tea.Cmd) {
	t := now()
	switch {
	case h.spent != "" && key == h.spent && t.Sub(h.last) <= h.wait():
		h.last = t
		return true, nil
	case h.running && key == h.key:
		h.seen(t)
		return true, nil
	case h.lapsed && key == h.key && h.repeats == 0 && t.Sub(h.start) <= lateWait:
		// The keyboard pauses longer than firstWait before repeating. Now
		// that pause is known, the hold goes on from where it began.
		h.seen(t)
		h.running, h.lapsed = true, false
		h.tag++
		return true, h.tick()
	}
	h.spent, h.lapsed = "", false
	return false, nil
}

// Update moves the hold on a tick of its own. done is true once the key was
// held for Duration; the hold then stops, and the key's further repeats are
// the hold's until it is let go.
func (h *Hold) Update(msg tea.Msg) (done bool, cmd tea.Cmd) {
	tm, ok := msg.(TickMsg)
	if !ok || tm.id != h.id || tm.tag != h.tag || !h.running {
		return false, nil
	}
	t := now()
	if t.Sub(h.last) > h.wait() {
		// Let go.
		h.running, h.lapsed = false, h.repeats == 0
		return false, nil
	}
	if h.repeats > 0 && t.Sub(h.start) >= h.Duration {
		h.running, h.spent = false, h.key
		return true, nil
	}
	return false, h.tick()
}

// seen records a repeat of the held key.
func (h *Hold) seen(t time.Time) {
	gap := t.Sub(h.last)
	switch {
	case h.repeats == 0:
		h.delay = t.Sub(h.start)
	case h.every == 0:
		h.every = gap
	default:
		h.every = (3*h.every + gap) / 4
	}
	h.repeats++
	h.last = t
}

// wait is how long the next repeat may take before the key counts as let go.
func (h *Hold) wait() time.Duration {
	switch {
	case h.repeats == 0 && h.delay > 0:
		return h.delay*5/4 + 50*time.Millisecond
	case h.repeats == 0:
		return firstWait
	case h.every == 0:
		return maxGap
	}
	return min(max(h.every*5/2, minGap), maxGap)
}

func (h *Hold) tick() tea.Cmd {
	id, tag := h.id, h.tag
	return tea.Tick(frame, func(time.Time) tea.Msg { return TickMsg{id, tag} })
}
