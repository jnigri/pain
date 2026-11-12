package reader

import (
	"strings"
	"testing"

	"pain/internal/markdown"
)

func TestSplitSections(t *testing.T) {
	words := func(n int) string { return strings.Repeat("word ", n) + "\n\n" }

	small := markdown.Parse([]byte("# Title\n\n" + words(50) + "## Two\n\n" + words(50)))
	if s := splitSections(small); len(s) != 1 || s[0].end != len(small) {
		t.Errorf("small file: %+v, want one section", s)
	}

	// The short intro joins the first section, and the short tail the last.
	big := markdown.Parse([]byte("# Title\n\n" + words(20) + "## One\n\n" + words(300) +
		"## Two\n\n" + words(300) + "## Three\n\n" + words(20)))
	s := splitSections(big)
	if len(s) != 2 {
		t.Fatalf("big file: %d sections, want 2: %+v", len(s), s)
	}
	if s[0].title != "Title · One" || s[1].title != "Two · Three" {
		t.Errorf("titles %q and %q", s[0].title, s[1].title)
	}
	if s[0].start != 0 || s[0].end != s[1].start || s[1].end != len(big) {
		t.Errorf("sections do not cover the file: %+v", s)
	}
	if big[s[1].start].Kind != markdown.Heading {
		t.Error("second section does not start at its heading")
	}
	if sectionOf(s, 0) != 0 || sectionOf(s, s[1].start) != 1 || sectionOf(s, len(big)) != 1 {
		t.Error("sectionOf")
	}
}

func TestParseQuiz(t *testing.T) {
	in := "```json\n" + `{"sections":[
		{"questions":[{"q":"Q1","choices":["a","b","c","d"],"answer":2,"why":"w"},
		              {"q":"bad","choices":["a"],"answer":0}]},
		{"questions":[]}]}` + "\n```"
	quiz, err := parseQuiz(in, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(quiz) != 3 || len(quiz[0]) != 1 || len(quiz[1]) != 0 || len(quiz[2]) != 0 {
		t.Fatalf("got %+v", quiz)
	}
	// Shuffling keeps the answer on the right choice.
	if q := quiz[0][0]; q.Choices[q.Answer] != "c" {
		t.Errorf("answer points to %q, want c", q.Choices[q.Answer])
	}
	if _, err := parseQuiz("sorry, no", 1); err == nil {
		t.Error("no error without JSON")
	}
	if _, err := parseQuiz(`{"sections":[{"questions":[]}]}`, 1); err == nil {
		t.Error("no error without questions")
	}
}
