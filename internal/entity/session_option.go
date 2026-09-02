package entity

type SessionOption struct {
	Text      string `json:"text"`
	IsCorrect bool   `json:"is_correct"`
	SortOrder int    `json:"sort_order"`
}
