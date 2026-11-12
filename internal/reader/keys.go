package reader

import "pain/internal/view"

var docKeys = []view.KeyRow{
	{Key: "backspace", Action: "delete one character"},
	{Key: "ctrl+w / alt+backspace", Action: "delete one word"},
	{Key: "enter", Action: "type the next space"},
	{Key: "ctrl+l", Action: "skip to the next sentence (block when underlined)"},
	{Key: "ctrl+h", Action: "back to the sentence (block) start, then the one before"},
	{Key: "ctrl+j / alt+enter", Action: "skip to the next screen line"},
	{Key: "ctrl+k", Action: "back to the line start, then the line before"},
	{Key: "tab", Action: "underline the whole block or the sentence"},
	{Key: "shift+tab", Action: "go back one block"},
	{Key: "↑ / ↓", Action: "scroll (pgup / pgdown for a page)"},
	{Key: "ctrl+a", Action: "ask Claude about the underlined sentence or block"},
	{Key: "ctrl+p", Action: "preferences"},
	{Key: "esc / ctrl+c", Action: "save your position and quit"},
}

// folderDocKeys replace the esc row of docKeys for a file opened from a folder.
var folderDocKeys = []view.KeyRow{
	{Key: "esc / ctrl+o", Action: "save your position and go back to the files"},
	{Key: "ctrl+c", Action: "save your position and quit"},
}

var askKeys = []view.KeyRow{
	{Key: "enter", Action: "send the question; on an answer, reply"},
	{Key: "tab", Action: "new question: sentence or whole block"},
	{Key: "n", Action: "on an answer, start a new conversation"},
	{Key: "ctrl+v", Action: "attach the image on the clipboard"},
	{Key: "← / →", Action: "move in the question; with alt, by word"},
	{Key: "ctrl+a / ctrl+e", Action: "go to the start / end of the question"},
	{Key: "ctrl+w / ctrl+k / ctrl+u", Action: "delete word / to the end / to the start"},
	{Key: "↑ / ↓", Action: "scroll (pgup / pgdown for a page)"},
	{Key: "ctrl+r", Action: "list the conversations about this file"},
	{Key: "g", Action: "on an answer or in the list, go to the block"},
	{Key: "esc", Action: "cancel a running question, or go back"},
}

// keysView lists every key of the screen you are on.
func (m *Model) keysView() string {
	title, rows := "keys", docKeys
	if m.Folder {
		rows = append(rows[:len(rows)-1:len(rows)-1], folderDocKeys...)
	}
	if m.ask != nil {
		title, rows = "keys · ask", askKeys
	}
	return view.KeysBox(m.w, m.h, title, rows)
}
