package plan

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"pain/internal/markdown"
)

// Check prints each plan in root's .reading-plans/ with what every row reads,
// so a plan can be checked without opening pain. For a heading anchor that
// matches nothing it lists the anchors the file has. It returns how many
// problems it found.
func Check(w io.Writer, root string) int {
	plans := Load(root)
	if len(plans) == 0 {
		fmt.Fprintf(w, "no plans in %s\n", filepath.Join(root, Dir))
		return 1
	}
	bad := 0
	for _, pl := range plans {
		var rows []string
		total, reads := 0, 0
		for _, it := range pl.Items {
			lead := strings.Repeat("  ", it.Depth) + it.Num + " " + it.Title
			if it.Group() {
				rows = append(rows, "  "+lead)
				continue
			}
			target := rel(root, it.Path)
			if it.Frag != "" {
				target += "#" + it.Frag
			}
			units, _, err := markdown.Load(it.Path)
			from, to, ok := markdown.Part(units, it.Frag)
			switch {
			case err != nil:
				bad++
				rows = append(rows, fmt.Sprintf("  %s\n      MISSING %s: %v", lead, target, unwrap(err)))
			case !ok:
				bad++
				rows = append(rows, fmt.Sprintf("  %s\n      MISSING %s: %s", lead, target, why(it)))
			default:
				words := 0
				for i := from; i < to; i++ {
					words += markdown.Words(&units[i])
				}
				total += words
				reads++
				rows = append(rows, fmt.Sprintf("  %s\n      %s · %d words", lead, target, words))
			}
		}
		fmt.Fprintf(w, "%s\n%s · %d rows to read · %d words\n", rel(root, pl.File), pl.Title, reads, total)
		if len(pl.Items) == 0 {
			bad++
			fmt.Fprintln(w, "  NO ROWS: the plan is the last list of the file")
		}
		for _, r := range rows {
			fmt.Fprintln(w, r)
		}
		fmt.Fprintln(w)
	}
	if bad > 0 {
		fmt.Fprintf(w, "%d problem(s)\n", bad)
	} else {
		fmt.Fprintln(w, "ok")
	}
	return bad
}

// why says what part of the file was not found.
func why(it Item) string {
	if _, _, isRange := markdown.LineRange(it.Frag); isRange || !markdown.IsMarkdown(it.Path) {
		return "no such lines, or not a line range (#L10-L40)"
	}
	units, _, err := markdown.Load(it.Path)
	if err != nil {
		return err.Error()
	}
	anchors := markdown.Anchors(units)
	return "no heading with that anchor; the file has: " + strings.Join(anchors, ", ")
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

func unwrap(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
