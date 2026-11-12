// Package app runs pain on a folder, switching between the folder view and the
// file opened from it.
package app

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"pain/internal/browser"
	"pain/internal/markdown"
	"pain/internal/reader"
	"pain/internal/store"
)

// app runs pain on a folder: the folder view, and the file opened from it.
type app struct {
	root string
	b    *browser.Model
	doc  *reader.Model // nil while in the folder view
	w, h int
}

func New(root string) (*app, error) {
	tree, err := browser.Scan(root, nil)
	if err != nil {
		return nil, err
	}
	return &app{root: root, b: browser.New(tree, store.ReadFolders()[root])}, nil
}

func (a *app) Init() tea.Cmd { return nil }

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
		a.b.W, a.b.H = msg.Width, msg.Height
	case reader.CloseMsg:
		path := a.doc.Path
		a.doc = nil
		a.back(path, msg.Finished)
		return a, nil
	}
	if a.doc != nil {
		_, cmd := a.doc.Update(msg)
		return a, cmd
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		open, quit := a.b.Update(key)
		if quit {
			return a, tea.Quit
		}
		if open != nil {
			a.open(open)
		}
	}
	return a, nil
}

// back shows the folder view on the file just left, or on the next unread
// one once it is finished.
func (a *app) back(path string, finished bool) {
	a.b.Refresh()
	n := a.b.Root.Find(path)
	if n == nil {
		return
	}
	a.b.Select(n)
	if finished {
		if u := a.b.NextUnread(n); u != nil {
			a.b.Select(u)
		}
	}
}

func (a *app) open(n *browser.Node) {
	src, err := os.ReadFile(n.Path)
	if err != nil {
		return
	}
	units := markdown.Parse(src)
	a.b.Last = n.Path
	store.SaveLastFile(a.root, n.Path)
	m := reader.New(n.Path, src, units, store.LoadProgress(n.Path, len(units)), store.LoadPrefs())
	if m.Finished {
		// Nothing to type in it.
		a.back(n.Path, true)
		return
	}
	m.Folder = true
	m.Update(tea.WindowSizeMsg{Width: a.w, Height: a.h})
	a.doc = m
}

func (a *app) View() string {
	if a.doc != nil {
		return a.doc.View()
	}
	return a.b.View()
}
