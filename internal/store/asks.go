package store

import "time"

type AskTurn struct {
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Images   []string `json:"images,omitempty"` // paths of the images sent with the question
}

// AskEntry is one conversation about a block, as saved in asks.json. Time is
// when it started and tells conversations apart.
type AskEntry struct {
	Unit      int       `json:"unit"`
	Block     string    `json:"block"`
	Selection string    `json:"selection,omitempty"` // the sentence asked about, empty for the whole block
	Section   bool      `json:"section,omitempty"`   // asked about the whole section the block sits in
	Turns     []AskTurn `json:"turns"`
	Time      time.Time `json:"time"`

	// Entries saved before conversations held a single question.
	Question string `json:"question,omitempty"`
	Answer   string `json:"answer,omitempty"`
}

func ReadAsks() map[string][]AskEntry {
	all := map[string][]AskEntry{}
	readCache("asks.json", &all)
	for _, es := range all {
		for i, e := range es {
			if len(e.Turns) == 0 && e.Question != "" {
				es[i].Turns = []AskTurn{{Question: e.Question, Answer: e.Answer}}
				es[i].Question, es[i].Answer = "", ""
			}
		}
	}
	return all
}

// SaveAsk stores a conversation, replacing its earlier copy if there is one.
func SaveAsk(path string, e AskEntry) {
	all := ReadAsks()
	es := all[path]
	for i := range es {
		if es[i].Time.Equal(e.Time) {
			es[i] = e
			writeCache("asks.json", all)
			return
		}
	}
	all[path] = append(es, e)
	writeCache("asks.json", all)
}
