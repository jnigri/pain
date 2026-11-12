package store

// Question is one multiple-choice question of a checkpoint.
type Question struct {
	Q       string   `json:"q"`
	Choices []string `json:"choices"`
	Answer  int      `json:"answer"` // index of the correct choice
	Why     string   `json:"why"`
}

// Quiz is the questions written for a file, per section, as saved in
// quizzes.json. Hash tells whether the file has changed since.
type Quiz struct {
	Hash     string       `json:"hash"`
	Sections [][]Question `json:"sections"`
}

// LoadQuiz returns the questions saved for a file, or nil when there are none
// for this content and section count.
func LoadQuiz(path, hash string, sections int) [][]Question {
	all := map[string]Quiz{}
	readCache("quizzes.json", &all)
	q := all[path]
	if q.Hash != hash || len(q.Sections) != sections {
		return nil
	}
	return q.Sections
}

func SaveQuiz(path, hash string, sections [][]Question) {
	all := map[string]Quiz{}
	readCache("quizzes.json", &all)
	all[path] = Quiz{Hash: hash, Sections: sections}
	writeCache("quizzes.json", all)
}
