package markdown

import (
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
)

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
	start := 0
	for i := min(pos, len(cs)) - 1; i >= 0; i-- {
		if sentenceEnds(cs, i) {
			start = i + 1
			break
		}
	}
	for start < len(cs) && cs[start].R == ' ' {
		start++
	}
	end := len(cs)
	for i := max(pos, start); i < len(cs); i++ {
		if sentenceEnds(cs, i) {
			end = i + 1
			break
		}
	}
	return start, end
}

// Abbreviations whose period does not end a sentence, written in lowercase
// without their last period: always, or only before a number ("Fig. 3").
var (
	abbrevAlways = map[string]bool{
		"e.g": true, "i.e": true, "eg": true, "ie": true, "cf": true, "vs": true,
		"viz": true, "approx": true, "mr": true, "mrs": true, "ms": true,
		"dr": true, "prof": true,
	}
	abbrevNumber = map[string]bool{
		"no": true, "nos": true, "fig": true, "figs": true, "eq": true,
		"eqs": true, "sec": true, "ch": true, "vol": true, "p": true, "pp": true,
		"ref": true, "art": true, "ex": true,
	}
)

// sentenceEnds tells whether a sentence ends at cell i: a ".", "!", "?" or
// "…", maybe followed by closing quotes or brackets, then a space or the end
// of the cells. It does not when the next word starts in lowercase ("approx.
// ten", "Yahoo! is"), after an abbreviation like "e.g." or "Fig. 3", or
// inside inline code.
func sentenceEnds(cs []Cell, i int) bool {
	if i+1 < len(cs) && cs[i+1].R != ' ' {
		return false
	}
	j := i
	for j >= 0 && strings.ContainsRune(`"')]”’»`, cs[j].R) {
		j--
	}
	if j < 0 || !strings.ContainsRune(".!?…", cs[j].R) || cs[j].Attr&InlineCode != 0 {
		return false
	}
	next := i + 1
	for next < len(cs) && cs[next].R == ' ' {
		next++
	}
	if next == len(cs) {
		return true
	}
	if n := cs[next]; unicode.IsLower(n.R) && n.Attr&InlineCode == 0 {
		return false
	}
	if cs[j].R != '.' {
		return true
	}
	w := j
	for w > 0 && (unicode.IsLetter(cs[w-1].R) || cs[w-1].R == '.') {
		w--
	}
	word := strings.ToLower(str(cs[w:j]))
	return !abbrevAlways[word] && !(abbrevNumber[word] && unicode.IsDigit(cs[next].R))
}

// str is the text of cells.
func str(cs []Cell) string {
	rs := make([]rune, len(cs))
	for i, c := range cs {
		rs[i] = c.R
	}
	return string(rs)
}
