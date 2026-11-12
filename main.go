package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"pain/internal/app"
	"pain/internal/markdown"
	"pain/internal/reader"
	"pain/internal/store"
)

func main() {
	restart := flag.Bool("restart", false, "ignore saved position and start from the top (files only)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: pain [-restart] FILE.md\n       pain DIR")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	path, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		fail(err)
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		a, err := app.New(path)
		if err != nil {
			fail(err)
		}
		if _, err := tea.NewProgram(a, tea.WithAltScreen()).Run(); err != nil {
			fail(err)
		}
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	units := markdown.Parse(src)

	start := 0
	if !*restart {
		start = store.LoadProgress(path, len(units))
	}
	m := reader.New(path, src, units, start, store.LoadPrefs())
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "pain:", err)
	os.Exit(1)
}
