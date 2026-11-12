package store

import "testing"

func TestActive(t *testing.T) {
	var p Prefs
	if p.IsActive("/notes/a.md") {
		t.Error("active with nothing listed")
	}
	if !p.ToggleActive("/notes", "/notes/a.md") {
		t.Error("toggle did not turn it on")
	}
	if !p.IsActive("/notes/sub/b.md") || !p.IsActive("/notes") {
		t.Error("folder does not cover what is under it")
	}
	if p.IsActive("/notes2/a.md") {
		t.Error("/notes covers /notes2")
	}
	// Turning it off from a file removes the folder that covers it.
	if p.ToggleActive("/notes/a.md", "/notes/a.md") || p.IsActive("/notes/a.md") {
		t.Error("toggle did not turn it off")
	}
}
