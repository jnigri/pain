package hold

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// clock makes now a clock moved by hand.
func clock(t *testing.T) func(time.Duration) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now = func() time.Time { return at }
	t.Cleanup(func() { now = time.Now })
	return func(d time.Duration) { at = at.Add(d) }
}

// tick is the tick the hold waits for now.
func (h *Hold) tickMsg() TickMsg { return TickMsg{h.id, h.tag} }

func TestHoldCompletes(t *testing.T) {
	wait := clock(t)
	h := New(time.Second)
	h.Start("enter")
	wait(400 * time.Millisecond)
	if mine, _ := h.Repeat("enter"); !mine {
		t.Fatal("the first repeat is not the hold's")
	}
	for range 20 {
		wait(30 * time.Millisecond)
		h.Repeat("enter")
	}
	if done, _ := h.Update(h.tickMsg()); !done {
		t.Fatalf("held for %v of %v, not done", now().Sub(h.start), h.Duration)
	}
	// The key that answered goes on repeating, and stays the hold's.
	wait(30 * time.Millisecond)
	if mine, _ := h.Repeat("enter"); !mine {
		t.Error("a repeat of the key that answered is not ignored")
	}
	wait(time.Second)
	if mine, _ := h.Repeat("enter"); mine {
		t.Error("a new press after letting go is ignored")
	}
}

func TestSinglePressNeverCompletes(t *testing.T) {
	wait := clock(t)
	h := New(time.Second)
	h.Start("1")
	wait(1500 * time.Millisecond)
	if done, _ := h.Update(h.tickMsg()); done || h.Running() {
		t.Error("a single press still holds")
	}
	if !h.Tapped() {
		t.Error("a single press is not a tap")
	}
	if h.Repeat("down"); h.Tapped() {
		t.Error("another key does not end the tap")
	}
}

func TestLetGoIsSeenFromThePace(t *testing.T) {
	wait := clock(t)
	h := New(time.Second)
	h.Start("1")
	wait(300 * time.Millisecond)
	h.Repeat("1")
	for range 5 {
		wait(30 * time.Millisecond)
		h.Repeat("1")
	}
	wait(90 * time.Millisecond)
	if h.Update(h.tickMsg()); !h.Running() {
		t.Fatal("let go within the repeat pace")
	}
	wait(30 * time.Millisecond)
	if h.Update(h.tickMsg()); h.Running() {
		t.Error("still held 120ms after repeats every 30ms")
	}
}

func TestLateFirstRepeatResumes(t *testing.T) {
	wait := clock(t)
	h := New(time.Second)
	h.Start("enter")
	wait(firstWait + time.Millisecond)
	h.Update(h.tickMsg())
	if h.Running() {
		t.Fatal("still running past the first wait")
	}
	wait(300 * time.Millisecond) // the first repeat, 1s after the press
	mine, cmd := h.Repeat("enter")
	if !mine || cmd == nil || !h.Running() {
		t.Fatal("a late first repeat does not take the hold up again")
	}
	// The pause is learnt: the next tap gives up a little after it.
	h.Cancel()
	h.Start("enter")
	wait(h.delay + 100*time.Millisecond)
	h.Update(h.tickMsg())
	if !h.Running() {
		t.Error("gave up before the learnt pause")
	}
}

func TestUnderline(t *testing.T) {
	line := "› \x1b[2m1  \x1b[0m\x1b[1mYes\x1b[0m"
	for _, p := range []float64{0, 0.5, 1} {
		got := Underline(line, 5, 20, p, DefaultLook)
		if w := ansi.StringWidth(got); w != 20 {
			t.Errorf("p=%v: width %d, want 20", p, w)
		}
		if plain := ansi.Strip(got); !strings.HasPrefix(plain, "› 1  Yes") {
			t.Errorf("p=%v: text %q", p, plain)
		}
	}
	// Half way, the second half of the columns after the lead is the track.
	got := Underline(line, 5, 15, 0.5, DefaultLook)
	if !strings.Contains(got, "\x1b[4:4m") {
		t.Error("no track half way")
	}
	if strings.Contains(Underline(line, 5, 15, 1, DefaultLook), "\x1b[4:4m") {
		t.Error("a track when full")
	}
}

func TestHint(t *testing.T) {
	got := Hint("› \x1b[1mYes\x1b[0m      ", 20, "hold")
	if plain := ansi.Strip(got); plain != "› Yes           hold" {
		t.Errorf("got %q", plain)
	}
	long := "› " + strings.Repeat("x", 16)
	if Hint(long, 20, "hold") != long {
		t.Error("a hint without room changes the line")
	}
}
