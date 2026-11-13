package store

import (
	"fmt"
	"time"
)

// Tally is what was typed and answered: in one session, in one file, or summed.
type Tally struct {
	Words   int     `json:"words"`
	Seconds float64 `json:"seconds"`
	// BestWPM is the fastest run of typing, in words per minute.
	BestWPM float64 `json:"best_wpm,omitempty"`
	// Checkpoint questions answered and answered right, every try counted.
	Answered int `json:"answered,omitempty"`
	Right    int `json:"right,omitempty"`
	// Explained counts the explanations Claude checked.
	Explained int `json:"explained,omitempty"`
}

func (t *Tally) Add(o Tally) {
	t.Words += o.Words
	t.Seconds += o.Seconds
	t.BestWPM = max(t.BestWPM, o.BestWPM)
	t.Answered += o.Answered
	t.Right += o.Right
	t.Explained += o.Explained
}

// WPM is the average typing speed in words per minute, or 0 until there is
// enough typing to go by.
func (t Tally) WPM() float64 {
	if t.Words < 20 || t.Seconds <= 0 {
		return 0
	}
	return float64(t.Words) / t.Seconds * 60
}

// TypingStats is the tally of every session, and of each file. The total gives
// the time left in the folder view.
type TypingStats struct {
	Tally
	Files map[string]Tally `json:"files,omitempty"`
}

func LoadTypingStats() TypingStats {
	var s TypingStats
	readCache("stats.json", &s)
	return s
}

// AddStats adds a session's tally to the total and to the file's.
func AddStats(path string, t Tally) {
	if t == (Tally{}) {
		return
	}
	s := LoadTypingStats()
	s.Add(t)
	if s.Files == nil {
		s.Files = map[string]Tally{}
	}
	f := s.Files[path]
	f.Add(t)
	s.Files[path] = f
	writeCache("stats.json", s)
}

// Under sums the files in dir and below.
func (s TypingStats) Under(dir string) Tally {
	var t Tally
	for p, f := range s.Files {
		if covers(dir, p) {
			t.Add(f)
		}
	}
	return t
}

// Clock writes a duration as 12m or 1h05m.
func Clock(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// TimeLeft estimates how long the remaining words take at your speed, or ""
// until there is enough typing to go by.
func (s TypingStats) TimeLeft(words int) string {
	if s.Words < 200 || s.Seconds <= 0 || words <= 0 {
		return ""
	}
	d := time.Duration(float64(words) / float64(s.Words) * s.Seconds * float64(time.Second))
	return "~" + Clock(max(d, time.Minute)) + " left"
}
