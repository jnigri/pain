package store

import (
	"fmt"
	"time"
)

// TypingStats is how fast you read by typing, summed over every session. It
// gives the time left in the folder view.
type TypingStats struct {
	Words   int     `json:"words"`
	Seconds float64 `json:"seconds"`
}

func LoadTypingStats() TypingStats {
	var s TypingStats
	readCache("stats.json", &s)
	return s
}

func AddTypingStats(words int, secs float64) {
	if words == 0 && secs == 0 {
		return
	}
	s := LoadTypingStats()
	s.Words += words
	s.Seconds += secs
	writeCache("stats.json", s)
}

// TimeLeft estimates how long the remaining words take at your speed, or ""
// until there is enough typing to go by.
func (s TypingStats) TimeLeft(words int) string {
	if s.Words < 200 || s.Seconds <= 0 || words <= 0 {
		return ""
	}
	d := time.Duration(float64(words) / float64(s.Words) * s.Seconds * float64(time.Second))
	if d < time.Hour {
		return fmt.Sprintf("~%dm left", max(int(d.Minutes()), 1))
	}
	return fmt.Sprintf("~%dh%02dm left", int(d.Hours()), int(d.Minutes())%60)
}
