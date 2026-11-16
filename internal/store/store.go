// Package store keeps pain's state between runs: JSON files in the user cache
// directory (progress.json, stats.json, asks.json, quizzes.json…) and the
// preferences in the config directory (see PrefsPath).
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Progress is a map of absolute file path to the unit you stopped at and
// the units read, wherever they were read from: the file itself or a reading
// plan's row for part of it. The unit count guards against resuming into a
// file that has changed.
type Position struct {
	Unit  int    `json:"unit"`
	Total int    `json:"total"`
	Done  bool   `json:"done,omitempty"` // every unit read; finishing the file starts it again at the top
	Read  Ranges `json:"read,omitempty"`
}

// Ranges are the units read, as sorted [from, to) pairs that do not touch.
type Ranges [][2]int

// Covered is the ranges read, including those of positions saved before
// pain kept them: everything before the unit you stopped at.
func (p Position) Covered() Ranges {
	switch {
	case p.Done:
		return Ranges{{0, p.Total}}
	case p.Read == nil && p.Unit > 0:
		return Ranges{{0, p.Unit}}
	}
	return p.Read
}

// Add is r with [from, to) read too.
func (r Ranges) Add(from, to int) Ranges {
	if from >= to {
		return r
	}
	var out Ranges
	for _, x := range r {
		switch {
		case x[1] < from:
			out = append(out, x)
		case x[0] > to:
			if from < to {
				out = append(out, [2]int{from, to})
				from, to = 0, 0
			}
			out = append(out, x)
		default:
			from, to = min(from, x[0]), max(to, x[1])
		}
	}
	if from < to {
		out = append(out, [2]int{from, to})
	}
	return out
}

// Remove is r with [from, to) not read.
func (r Ranges) Remove(from, to int) Ranges {
	var out Ranges
	for _, x := range r {
		if x[0] < from {
			out = append(out, [2]int{x[0], min(x[1], from)})
		}
		if x[1] > to {
			out = append(out, [2]int{max(x[0], to), x[1]})
		}
	}
	return out
}

// Has tells whether every unit of [from, to) is read.
func (r Ranges) Has(from, to int) bool {
	for _, x := range r {
		if x[0] <= from && to <= x[1] {
			return true
		}
	}
	return from >= to
}

// FirstGap is the first unit of [from, to) not read, or from when all are.
func (r Ranges) FirstGap(from, to int) int {
	at := from
	for _, x := range r {
		if x[0] <= at && at < x[1] {
			at = x[1]
		}
	}
	if at >= to {
		return from
	}
	return at
}

// CachePath is where pain keeps its state files, such as progress.json.
func CachePath(name string) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "pain", name)
}

func readCache(name string, v any) {
	if data, err := os.ReadFile(CachePath(name)); err == nil {
		_ = json.Unmarshal(data, v)
	}
}

// writeCache saves v as JSON. It writes to a temporary file and renames it
// into place, so a crash mid-write never leaves a truncated file behind.
func writeCache(name string, v any) {
	f := CachePath(name)
	if f == "" {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	dir := filepath.Dir(f)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), f) != nil {
		_ = os.Remove(tmp.Name())
	}
}

func ReadProgress() map[string]Position {
	all := map[string]Position{}
	readCache("progress.json", &all)
	return all
}

func writeProgress(all map[string]Position) { writeCache("progress.json", all) }

func LoadProgress(path string, total int) int {
	p, ok := ReadProgress()[path]
	if !ok || p.Total != total || p.Unit >= total {
		return 0
	}
	return p.Unit
}

// LoadPart is where to start reading units [from, to) of a file: the first
// one not read yet, or from when all are.
func LoadPart(path string, total, from, to int) int {
	p, ok := ReadProgress()[path]
	if !ok || p.Total != total {
		return from
	}
	return p.Covered().FirstGap(from, to)
}

// SaveProgress saves where you stopped in a file, and counts everything
// before it as read.
func SaveProgress(path string, unit, total int) {
	update(path, total, func(p *Position) {
		p.Unit = unit
		p.Read = p.Read.Add(0, unit)
	})
}

// MarkDone counts the whole file as read; opening it again starts at the
// top.
func MarkDone(path string, total int) {
	update(path, total, func(p *Position) {
		p.Unit = 0
		p.Read = p.Read.Add(0, total)
	})
}

// MarkRead counts units [from, to) of a file as read, as reading a part of
// it from a reading plan does.
func MarkRead(path string, total, from, to int) {
	update(path, total, func(p *Position) { p.Read = p.Read.Add(from, to) })
}

// ClearRead counts units [from, to) of a file as not read.
func ClearRead(path string, total, from, to int) {
	update(path, total, func(p *Position) {
		p.Read = p.Read.Remove(from, to)
		if p.Unit > from {
			p.Unit = from
		}
	})
}

// update changes a file's position, starting afresh when the file has
// changed shape, and keeps Done in step with what is read.
func update(path string, total int, fn func(*Position)) {
	all := ReadProgress()
	p, ok := all[path]
	if !ok || p.Total != total {
		p = Position{Total: total}
	}
	p.Read, p.Done = p.Covered(), false
	fn(&p)
	p.Done = p.Read.Has(0, total)
	all[path] = p
	writeProgress(all)
}

func ClearProgress(path string) {
	all := ReadProgress()
	delete(all, path)
	writeProgress(all)
}

// Folders is a map of folder path to the file you opened last in it.
func ReadFolders() map[string]string {
	all := map[string]string{}
	readCache("folders.json", &all)
	return all
}

func SaveLastFile(root, path string) {
	all := ReadFolders()
	all[root] = path
	writeCache("folders.json", all)
}
