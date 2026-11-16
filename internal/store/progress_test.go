package store

import (
	"reflect"
	"testing"
)

func TestRanges(t *testing.T) {
	var r Ranges
	r = r.Add(5, 8).Add(0, 2).Add(10, 12)
	if want := (Ranges{{0, 2}, {5, 8}, {10, 12}}); !reflect.DeepEqual(r, want) {
		t.Fatalf("add: %v", r)
	}
	if got := r.Add(7, 10); !reflect.DeepEqual(got, Ranges{{0, 2}, {5, 12}}) {
		t.Errorf("join: %v", got)
	}
	if got := r.Add(2, 5); !reflect.DeepEqual(got, Ranges{{0, 8}, {10, 12}}) {
		t.Errorf("touching: %v", got)
	}
	if got := r.Add(3, 4); !reflect.DeepEqual(got, Ranges{{0, 2}, {3, 4}, {5, 8}, {10, 12}}) {
		t.Errorf("between: %v", got)
	}
	if got := r.Remove(1, 6); !reflect.DeepEqual(got, Ranges{{0, 1}, {6, 8}, {10, 12}}) {
		t.Errorf("remove: %v", got)
	}
	if !r.Has(5, 8) || !r.Has(6, 7) || r.Has(4, 8) || r.Has(7, 11) || !r.Has(3, 3) {
		t.Error("has")
	}
	for _, c := range [][3]int{{0, 12, 2}, {5, 9, 8}, {5, 8, 5}, {3, 5, 3}, {10, 12, 10}} {
		if got := r.FirstGap(c[0], c[1]); got != c[2] {
			t.Errorf("FirstGap(%d, %d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestSharedProgress(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	const f, n = "/doc.md", 20
	// A part read from a plan counts for the file.
	MarkRead(f, n, 10, 15)
	if p := ReadProgress()[f]; p.Done || !p.Covered().Has(10, 15) || LoadProgress(f, n) != 0 {
		t.Fatalf("%+v", p)
	}
	if got := LoadPart(f, n, 10, 18); got != 15 {
		t.Errorf("part resumes at %d", got)
	}
	// Reading the file from the top joins it.
	SaveProgress(f, 12, n)
	if p := ReadProgress()[f]; !reflect.DeepEqual(p.Read, Ranges{{0, 15}}) || LoadProgress(f, n) != 12 {
		t.Errorf("%+v", p)
	}
	MarkRead(f, n, 15, n)
	if p := ReadProgress()[f]; !p.Done {
		t.Errorf("every part read but not done: %+v", p)
	}
	// Reading it again keeps it read and resumes.
	SaveProgress(f, 3, n)
	if p := ReadProgress()[f]; !p.Done || LoadProgress(f, n) != 3 {
		t.Errorf("%+v", p)
	}
	ClearRead(f, n, 0, 5)
	if p := ReadProgress()[f]; p.Done || p.Unit != 0 || !reflect.DeepEqual(p.Read, Ranges{{5, n}}) {
		t.Errorf("%+v", p)
	}
	// A changed file starts afresh; an old position counts what is before it.
	MarkRead(f, n+1, 0, 1)
	if p := ReadProgress()[f]; !reflect.DeepEqual(p.Read, Ranges{{0, 1}}) {
		t.Errorf("%+v", p)
	}
	all := ReadProgress()
	all["/old.md"] = Position{Unit: 4, Total: 9}
	writeProgress(all)
	MarkRead("/old.md", 9, 6, 7)
	if p := ReadProgress()["/old.md"]; !reflect.DeepEqual(p.Read, Ranges{{0, 4}, {6, 7}}) {
		t.Errorf("%+v", p)
	}
}
