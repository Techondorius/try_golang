package task

import (
	"strings"
	"unicode/utf8"
)

// Title is an immutable value object: equality depends only on its value.
type Title struct{ value string }

func NewTitle(raw string) (Title, error) {
	title := strings.TrimSpace(raw)
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 200 {
		return Title{}, ErrInvalidTitle
	}
	return Title{value: title}, nil
}
func (t Title) String() string { return t.value }
