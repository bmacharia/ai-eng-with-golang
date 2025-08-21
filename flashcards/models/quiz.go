package models

type Message struct {
	Role     string        `json:"role"` // "user" or "assistant"
	Content  string        `json:"content"`
	Question *QuestionData `json:"question,omitempty"`
}

type QuestionData struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Type          string   `json:"type"` // "multiple-choice", "essay", etc.
	Options       []string `json:"options,omitempty"`
	CorrectAnswer string   `json:"correctAnswer,omitempty"`
	Explanation   string   `json:"explanation,omitempty"`
	Difficulty    string   `json:"difficulty"`
	BasedOnNotes  []int    `json:"basedOnNotes"`
}
