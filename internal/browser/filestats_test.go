package browser

import (
	"testing"

	"pain/internal/markdown"
	"pain/internal/store"
)

func TestFileStats(t *testing.T) {
	units := markdown.Parse([]byte("# Title here\n\nOne two three.\n\n---\n\nFour.\n"))
	fs := newFileStats(units)
	if got := fs.total(); got != 6 {
		t.Fatalf("total = %d, want 6", got)
	}
	n := fs.units()
	tests := []struct {
		name string
		p    store.Position
		ok   bool
		want int
	}{
		{"unsaved", store.Position{}, false, 0},
		{"done", store.Position{Done: true, Total: n}, true, 6},
		{"start", store.Position{Unit: 0, Total: n}, true, 0},
		{"after heading", store.Position{Unit: 1, Total: n}, true, 2},
		{"file changed", store.Position{Unit: 1, Total: n + 1}, true, 0},
	}
	for _, tt := range tests {
		if got := fs.read(tt.p, tt.ok); got != tt.want {
			t.Errorf("%s: read = %d, want %d", tt.name, got, tt.want)
		}
	}
}
