package markdown

import "github.com/mattn/go-runewidth"

// Wrap splits cells into line ranges no wider than w. Words move whole to the
// next line; spaces stay at the end of the line they follow.
func Wrap(cs []Cell, w int, hard bool) [][2]int {
	w = max(w, 8)
	var out [][2]int
	start, col := 0, 0
	if hard {
		for i, c := range cs {
			cw := runewidth.RuneWidth(c.R)
			if col+cw > w && i > start {
				out = append(out, [2]int{start, i})
				start, col = i, 0
			}
			col += cw
		}
		return append(out, [2]int{start, len(cs)})
	}
	for i := 0; i < len(cs); {
		j := i
		for j < len(cs) && cs[j].R != ' ' {
			j++
		}
		wordEnd := j
		for j < len(cs) && cs[j].R == ' ' {
			j++
		}
		ww := Width(cs[i:wordEnd])
		if col > 0 && col+ww > w {
			out = append(out, [2]int{start, i})
			start, col = i, 0
		}
		if ww > w {
			for k := i; k < wordEnd; k++ {
				cw := runewidth.RuneWidth(cs[k].R)
				if col+cw > w && col > 0 {
					out = append(out, [2]int{start, k})
					start, col = k, 0
				}
				col += cw
			}
		} else {
			col += ww
		}
		col += Width(cs[wordEnd:j])
		i = j
	}
	return append(out, [2]int{start, len(cs)})
}

// Sentence returns the cell range of the sentence holding pos.
func Sentence(cs []Cell, pos int) (int, int) {
	ends := func(i int) bool {
		r := cs[i].R
		return (r == '.' || r == '!' || r == '?') && (i+1 == len(cs) || cs[i+1].R == ' ')
	}
	start := 0
	for i := min(pos, len(cs)) - 1; i >= 0; i-- {
		if ends(i) {
			start = i + 1
			break
		}
	}
	for start < len(cs) && cs[start].R == ' ' {
		start++
	}
	end := len(cs)
	for i := max(pos, start); i < len(cs); i++ {
		if ends(i) {
			end = i + 1
			break
		}
	}
	return start, end
}
