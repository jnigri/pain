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

// Score is how the questions of one section went the last time they were
// answered to the end. Total is 0 until then.
type Score struct {
	Right int `json:"right"`
	Total int `json:"total"`
}

// FileScores is the scores of a file, per section, as saved in scores.json.
type FileScores struct {
	Hash     string  `json:"hash"`
	Sections []Score `json:"sections"`
}

// Sum adds up the sections answered.
func (f FileScores) Sum() Score {
	var s Score
	for _, x := range f.Sections {
		s.Right += x.Right
		s.Total += x.Total
	}
	return s
}

func ReadScores() map[string]FileScores {
	all := map[string]FileScores{}
	readCache("scores.json", &all)
	return all
}

// LoadScores returns the score of each section of a file, empty for a file
// that has changed since.
func LoadScores(path, hash string, sections int) []Score {
	f := ReadScores()[path]
	if f.Hash != hash || len(f.Sections) != sections {
		return make([]Score, sections)
	}
	return f.Sections
}

func SaveScores(path, hash string, sections []Score) {
	all := ReadScores()
	all[path] = FileScores{Hash: hash, Sections: sections}
	writeCache("scores.json", all)
}
