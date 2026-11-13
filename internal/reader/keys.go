package reader

import "pain/internal/view"

var docKeys = []view.KeyRow{
	{Key: "tab / shift+tab", Action: "widen / narrow the selection: sentence, block, section"},
	{Key: "ctrl+l", Action: "skip the rest of the selection"},
	{Key: "ctrl+h", Action: "back to the start of the selection, then the one before"},
	{Key: "ctrl+j / ctrl+k", Action: "the same by screen line (alt+enter skips one too)"},
	{Key: "ctrl+a", Action: "ask Claude about the selection"},
	{Key: "ctrl+e", Action: "explain the selection in your own words; Claude checks it"},
	{Key: "ctrl+g", Action: "answer the questions of this section now, or again"},
	{Key: "ctrl+t", Action: "active reading on or off: a checkpoint ends each section"},
	{Key: "backspace", Action: "delete one character (ctrl+w / alt+backspace: one word)"},
	{Key: "enter", Action: "type the next space"},
	{Key: "↑ / ↓", Action: "scroll (pgup / pgdown for a page)"},
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
	{Key: "tab", Action: "new question: about the sentence, the block or the section"},
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

var quizKeys = []view.KeyRow{
	{Key: "↑ / ↓ / j / k", Action: "move between the choices"},
	{Key: "enter / space", Action: "answer with the choice selected, then go on"},
	{Key: "1 – 9", Action: "answer with that choice"},
	{Key: "r", Action: "on the score, answer the same questions again"},
	{Key: "ctrl+r", Action: "have Claude write new questions for the section"},
	{Key: "ctrl+e / tab", Action: "explain the section in your own words instead"},
	{Key: "esc", Action: "skip the checkpoint"},
}

var reviewKeys = []view.KeyRow{
	{Key: "enter", Action: "send your explanation; on the feedback, reply or try again"},
	{Key: "tab", Action: "at a checkpoint, before sending: answer questions instead"},
	{Key: "ctrl+v", Action: "attach the image on the clipboard"},
	{Key: "← / →", Action: "move in the text; with alt, by word"},
	{Key: "↑ / ↓", Action: "scroll (pgup / pgdown for a page)"},
	{Key: "esc", Action: "cancel a running check, skip, or carry on reading"},
}

// keysView lists every key of the screen you are on.
func (m *Model) keysView() string {
	title, rows := "keys", docKeys
	if m.Folder {
		rows = append(rows[:len(rows)-1:len(rows)-1], folderDocKeys...)
	}
	switch {
	case m.quiz != nil:
		title, rows = "keys · checkpoint", quizKeys
	case m.ask != nil && m.ask.review:
		title, rows = "keys · explain", reviewKeys
	case m.ask != nil:
		title, rows = "keys · ask", askKeys
	}
	return view.KeysBox(m.w, m.h, title, rows)
}
