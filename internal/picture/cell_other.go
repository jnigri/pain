//go:build !unix

package picture

// cellSize is a guess where the terminal cannot be asked.
func cellSize() (w, h int, known bool) { return 10, 20, false }
