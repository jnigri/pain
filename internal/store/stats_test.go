package store

import "testing"

func TestTimeLeft(t *testing.T) {
	s := TypingStats{Words: 600, Seconds: 600} // one word per second
	if got := s.TimeLeft(120); got != "~2m left" {
		t.Errorf("timeLeft(120) = %q", got)
	}
	if got := s.TimeLeft(3900); got != "~1h05m left" {
		t.Errorf("timeLeft(3900) = %q", got)
	}
	if got := (TypingStats{Words: 10, Seconds: 10}).TimeLeft(100); got != "" {
		t.Errorf("too little data should give empty, got %q", got)
	}

}
