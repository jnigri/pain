// Package store keeps pain's state between runs: JSON files in the user cache
// directory (progress.json, stats.json, asks.json, quizzes.json…) and the
// preferences in the config directory (see PrefsPath).
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Progress is a map of absolute file path to the unit you stopped at. The
// unit count guards against resuming into a file that has changed.
type Position struct {
	Unit  int  `json:"unit"`
	Total int  `json:"total"`
	Done  bool `json:"done,omitempty"` // read to the end; reopening starts at the top
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
	if !ok || p.Done || p.Total != total || p.Unit >= total {
		return 0
	}
	return p.Unit
}

func SaveProgress(path string, unit, total int) {
	all := ReadProgress()
	all[path] = Position{Unit: unit, Total: total}
	writeProgress(all)
}

func MarkDone(path string, total int) {
	all := ReadProgress()
	all[path] = Position{Total: total, Done: true}
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
