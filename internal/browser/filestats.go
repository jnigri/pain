package browser

import (
	"pain/internal/markdown"
	"pain/internal/store"
)

// fileStats holds a file's words before each unit, so the words read up to a
// saved position is a lookup. words has one entry per unit plus the total.
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
func (f fileStats) total() int { return f.words[len(f.words)-1] }

// read is the words behind a saved position. A position saved for a file that
// has changed shape counts as nothing, as it does when resuming.
func (f fileStats) read(p store.Position, ok bool) int {
	switch {
	case !ok:
		return 0
	case p.Done:
		return f.total()
	case p.Total != f.units() || p.Unit >= f.units():
		return 0
	}
	return f.words[p.Unit]
}
