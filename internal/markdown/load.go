package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// IsMarkdown tells whether path is read as Markdown rather than as code.
func IsMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

// Load reads a file and parses it. A Markdown file is parsed as such; any
// other file is shown as one code block named after its extension.
func Load(path string) (units []Unit, src []byte, err error) {
	src, err = os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if !IsMarkdown(path) {
		return Code(src, strings.TrimPrefix(filepath.Ext(path), ".")), src, nil
	}
	return ParseIn(src, filepath.Dir(path)), src, nil
}

// Part finds the units [from, to) a fragment picks: a heading anchor
// ("applications") or a line range ("L10-L40"). No fragment picks them all.
// ok is false when frag matches nothing.
func Part(units []Unit, frag string) (from, to int, ok bool) {
	if frag == "" {
		return 0, len(units), true
	}
	first, last, ranged := LineRange(frag)
	if !ranged {
		return Section(units, frag)
	}
	from, to = len(units), len(units)
	for i, u := range units {
		if u.Line >= first && from == len(units) && u.Kind != Spacer {
			from = i
		}
		if u.Line > last {
			to = i
			break
		}
	}
	for to > from && units[to-1].Kind == Spacer {
		to--
	}
	return from, to, from < to
}

// Code parses src as one code block in language lang. Each line keeps its
// line number; the language label has none.
func Code(src []byte, lang string) []Unit {
	s := strings.TrimRight(string(src), "\n")
	// The fence must be longer than any run of backticks in the code.
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	units := Parse([]byte(fmt.Sprintf("%s%s\n%s\n%s\n", fence, lang, s, fence)))
	for i := range units {
		units[i].Line-- // the fence line comes first
	}
	if lang != "" && len(units) > 0 {
		units[0].Line = 0
	}
	return units
}

// LineRange reads a fragment such as "L10-L40" or "L10", lines counted
// from 1. ok is false when frag is not a line range.
func LineRange(frag string) (from, to int, ok bool) {
	a, b, isRange := strings.Cut(frag, "-")
	from, ok = lineNum(a)
	if !ok {
		return 0, 0, false
	}
	if !isRange {
		return from, from, true
	}
	to, ok = lineNum(b)
	if !ok || to < from {
		return 0, 0, false
	}
	return from, to, true
}

func lineNum(s string) (int, bool) {
	if len(s) < 2 || (s[0] != 'L' && s[0] != 'l') {
		return 0, false
	}
	n, err := strconv.Atoi(s[1:])
	return n, err == nil && n > 0
}

// Slug is the anchor GitHub gives a heading: lower case, punctuation
// dropped, spaces turned into hyphens.
func Slug(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			sb.WriteRune(r)
		case r == ' ':
			sb.WriteRune('-')
		}
	}
	return sb.String()
}

// Anchors are the anchors of the headings in units, as GitHub makes them,
// in order. Repeated headings take "-1", "-2"…
func Anchors(units []Unit) []string {
	var out []string
	seen := map[string]int{}
	for _, u := range units {
		if u.Kind != Heading {
			continue
		}
		var sb strings.Builder
		for _, c := range u.Cells {
			sb.WriteRune(c.R)
		}
		base := Slug(strings.TrimSpace(sb.String()))
		slug := base
		if n := seen[base]; n > 0 {
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		seen[base]++
		out = append(out, slug)
	}
	return out
}

// Section finds the heading whose anchor is anchor and returns the units
// from it to the next heading of the same or a higher level. Runs of
// hyphens are not told apart, so "a-+-b" can be written "a-b".
func Section(units []Unit, anchor string) (from, to int, ok bool) {
	want := squash(strings.ToLower(anchor))
	anchors := Anchors(units)
	h := -1
	for i, u := range units {
		if u.Kind != Heading {
			continue
		}
		h++
		if squash(anchors[h]) != want {
			continue
		}
		to = len(units)
		for j := i + 1; j < len(units); j++ {
			if units[j].Kind == Heading && units[j].Level <= u.Level {
				to = j
				break
			}
		}
		for to > i+1 && units[to-1].Kind == Spacer {
			to--
		}
		return i, to, true
	}
	return 0, 0, false
}

// squash turns runs of hyphens into one.
func squash(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}
