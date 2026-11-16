package browser

import (
	"pain/internal/markdown"
	"pain/internal/store"
)

// fileStats holds a file's words before each unit, so the words in any run
// of units is a lookup. words has one entry per unit plus the total.
type fileStats struct {
	words []int
}

func newFileStats(units []markdown.Unit) fileStats {
	words := make([]int, len(units)+1)
	for i := range units {
		words[i+1] = words[i] + markdown.Words(&units[i])
	}
	return fileStats{words}
}

func (f fileStats) units() int { return len(f.words) - 1 }

// in is the words of units [from, to).
func (f fileStats) in(from, to int) int { return f.words[to] - f.words[from] }

// read is the words read among units [from, to), r being the ranges read.
func (f fileStats) read(r store.Ranges, from, to int) int {
	n := 0
	for _, x := range r {
		a, b := max(x[0], from), min(x[1], to, f.units())
		if a < b {
			n += f.in(a, b)
		}
	}
	return n
}
