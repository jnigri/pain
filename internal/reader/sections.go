package reader

import (
	"fmt"
	"strings"

	"pain/internal/markdown"
)

// A file under smallFileWords is one section. Longer ones are cut at headings
// and rules, into sections of at least minSectionWords.
const (
	smallFileWords  = 500
	minSectionWords = 150
)

// section is a run of units, [start, end), that ends with a checkpoint.
type section struct {
	start, end int
	title      string
}

func cellText(cs []markdown.Cell) string {
	rs := make([]rune, len(cs))
	for i, c := range cs {
		rs[i] = c.R
	}
	return string(rs)
}

// last is the last unit of the section that shows something, where its
// checkpoint is drawn.
func (s section) last(units []markdown.Unit) int {
	for i := s.end - 1; i > s.start; i-- {
		if k := units[i].Kind; k != markdown.Spacer && k != markdown.Rule {
			return i
		}
	}
	return s.start
}

// splitSections cuts a file into the sections checkpoints are about.
func splitSections(units []markdown.Unit) []section {
	words := make([]int, len(units))
	total := 0
	for i := range units {
		words[i] = markdown.Words(&units[i])
		total += words[i]
	}
	var out []section
	if total >= smallFileWords {
		start, n := 0, 0
		for i := range units {
			cut := units[i].Kind == markdown.Heading || (i > 0 && units[i-1].Kind == markdown.Rule)
			if cut && n >= minSectionWords {
				out = append(out, section{start: start, end: i})
				start, n = i, 0
			}
			n += words[i]
		}
		// A short tail joins the section before it.
		if n < minSectionWords && len(out) > 0 {
			out[len(out)-1].end = len(units)
		} else {
			out = append(out, section{start: start, end: len(units)})
		}
	} else {
		out = []section{{end: len(units)}}
	}
	for i := range out {
		out[i].title = sectionTitle(units[out[i].start:out[i].end], i)
	}
	return out
}

// sectionTitle names a section after its headings, or its rank without any.
func sectionTitle(units []markdown.Unit, i int) string {
	var heads []string
	for k := range units {
		if units[k].Kind != markdown.Heading {
			continue
		}
		if len(heads) == 3 {
			heads = append(heads, "…")
			break
		}
		heads = append(heads, strings.TrimSpace(cellText(units[k].Cells)))
	}
	if len(heads) == 0 {
		return fmt.Sprintf("part %d", i+1)
	}
	return strings.Join(heads, " · ")
}

// sectionOf is the section unit i sits in.
func sectionOf(secs []section, i int) int {
	for s := range secs {
		if i < secs[s].end {
			return s
		}
	}
	return len(secs) - 1
}

// sectionText is a section as plain text, for Claude.
func sectionText(units []markdown.Unit, s section) string {
	var sb strings.Builder
	for i := s.start; i < s.end; i++ {
		u := &units[i]
		switch u.Kind {
		case markdown.Spacer, markdown.Rule:
			sb.WriteString("\n")
		case markdown.Image:
			sb.WriteString("[image: " + blockText(u) + "]\n")
		case markdown.Heading:
			sb.WriteString(strings.Repeat("#", max(u.Level, 1)) + " " + blockText(u) + "\n")
		default:
			sb.WriteString(blockText(u) + "\n")
		}
	}
	return strings.TrimSpace(sb.String())
}
