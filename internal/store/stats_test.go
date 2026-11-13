package store

import "testing"

func TestTimeLeft(t *testing.T) {
	s := TypingStats{Tally: Tally{Words: 600, Seconds: 600}} // one word per second
	if got := s.TimeLeft(120); got != "~2m left" {
		t.Errorf("timeLeft(120) = %q", got)
	}
	if got := s.TimeLeft(3900); got != "~1h05m left" {
		t.Errorf("timeLeft(3900) = %q", got)
	}
	if got := s.TimeLeft(10); got != "~1m left" {
		t.Errorf("timeLeft(10) = %q", got)
	}
	if got := (TypingStats{Tally: Tally{Words: 10, Seconds: 10}}).TimeLeft(100); got != "" {
		t.Errorf("too little data should give empty, got %q", got)
	}
}

func TestUnder(t *testing.T) {
	s := TypingStats{Files: map[string]Tally{
		"/notes/a.md":     {Words: 100, Seconds: 120, BestWPM: 60, Answered: 2, Right: 1},
		"/notes/sub/b.md": {Words: 50, Seconds: 30, BestWPM: 80},
		"/notes2/c.md":    {Words: 1000},
	}}
	got := s.Under("/notes")
	if got.Words != 150 || got.Seconds != 150 || got.BestWPM != 80 || got.Answered != 2 || got.Right != 1 {
		t.Errorf("Under(/notes) = %+v", got)
	}
	if w := got.WPM(); w != 60 {
		t.Errorf("WPM = %v, want 60", w)
	}
}
