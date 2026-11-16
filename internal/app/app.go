// Package app runs pain on a folder, switching between the folder view and the
// file opened from it.
package app

import (
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
	// The entry the doc was opened from: a file can be both in the folder and
	// in a reading plan.
	opened *browser.Node
	w, h   int
}

func New(root string) (*app, error) {
	tree, err := browser.Load(root)
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
		a.doc = nil
		a.back(a.opened, msg.Finished)
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
			return a, a.open(open)
		}
	}
	return a, nil
}

// back shows the folder view on the file just left, or on the next unread
// one once it is finished. In a reading plan that is the plan's next row.
func (a *app) back(n *browser.Node, finished bool) {
	a.b.Refresh()
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

func (a *app) open(n *browser.Node) tea.Cmd {
	units, src, ok, err := markdown.Load(n.Path, n.Frag)
	if err != nil || !ok {
		return nil
	}
	a.opened = n
	a.b.Last = n.Key
	store.SaveLastFile(a.root, n.Key)
	m := reader.New(n.Key, src, units, store.LoadProgress(n.Key, len(units)), store.LoadPrefs())
	if m.Finished {
		// Nothing to type in it.
		a.back(n, true)
		return nil
	}
	m.Folder, m.Scope, m.File = true, a.root, n.Path
	m.Update(tea.WindowSizeMsg{Width: a.w, Height: a.h})
	a.doc = m
	return m.Init()
}

func (a *app) View() string {
	if a.doc != nil {
		return a.doc.View()
	}
	return a.b.View()
}
