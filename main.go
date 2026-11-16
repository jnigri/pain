package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"pain/internal/app"
	"pain/internal/markdown"
	"pain/internal/picture"
	"pain/internal/plan"
	"pain/internal/reader"
	"pain/internal/store"
	"pain/internal/theme"
)

func main() {
	restart := flag.Bool("restart", false, "ignore saved position and start from the top (files only)")
	check := flag.Bool("check", false, "print the reading plans of DIR, row by row, and exit 1 if a row is not found")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: pain [-restart] FILE.md\n       pain DIR\n       pain -check DIR")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *check {
		// Before the terminal is touched: this runs from scripts and agents.
		path, err := filepath.Abs(flag.Arg(0))
		if err != nil {
			fail(err)
		}
		if plan.Check(os.Stdout, path) > 0 {
			os.Exit(1)
		}
		return
	}

	theme.Detect()
	theme.Watch()
	defer theme.Unwatch()

	path, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		fail(err)
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		a, err := app.New(path)
		if err != nil {
			fail(err)
		}
		_, err = tea.NewProgram(a, tea.WithAltScreen(), tea.WithFilter(theme.Filter)).Run()
		picture.Release()
		if err != nil {
			fail(err)
		}
		return
	}
	src, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	units := markdown.ParseIn(src, filepath.Dir(path))

	start := 0
	if !*restart {
		start = store.LoadProgress(path, len(units))
	}
	m := reader.New(path, src, units, start, store.LoadPrefs())
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithFilter(theme.Filter)).Run()
	picture.Release()
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	theme.Unwatch()
	fmt.Fprintln(os.Stderr, "pain:", err)
	os.Exit(1)
}
