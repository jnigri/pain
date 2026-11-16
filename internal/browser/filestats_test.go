package browser

import (
	"testing"

	"pain/internal/markdown"
	"pain/internal/store"
)

func TestFileStats(t *testing.T) {
	units := markdown.Parse([]byte("# Title here\n\nOne two three.\n\n---\n\nFour.\n"))
	fs := newFileStats(units)
	n := fs.units()
	if got := fs.in(0, n); got != 6 {
		t.Fatalf("total = %d, want 6", got)
	}
	tests := []struct {
		name     string
		r        store.Ranges
		from, to int
		want     int
	}{
		{"nothing read", nil, 0, n, 0},
		{"all read", store.Ranges{{0, n}}, 0, n, 6},
		{"heading read", store.Ranges{{0, 1}}, 0, n, 2},
		{"part of what is read", store.Ranges{{0, n}}, 2, n, 4},
		{"read outside the part", store.Ranges{{0, 1}}, 2, n, 0},
		{"two runs", store.Ranges{{0, 1}, {2, 3}, {6, n + 3}}, 0, n, 6},
	}
	for _, tt := range tests {
		if got := fs.read(tt.r, tt.from, tt.to); got != tt.want {
			t.Errorf("%s: read = %d, want %d", tt.name, got, tt.want)
		}
	}
}
