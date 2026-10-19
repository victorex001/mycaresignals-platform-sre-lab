package ai

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// SourceDocument is the minimal document representation used by theme extraction.
type SourceDocument struct {
	DocType string
	Title   string
	Text    string
}

// DocumentSource describes whether documents belong to an existing placement or referral.
type DocumentSource string

const (
	DocumentSourceExisting DocumentSource = "existing_child"
	DocumentSourceReferral DocumentSource = "new_referral"
)

func buildDocsContext(docs []SourceDocument) string {
	var parts []string
	for _, d := range docs {
		header := fmt.Sprintf("[%s] %s\n", strings.ToUpper(d.DocType), safeTitle(d.Title))
		body := truncateRunes(d.Text, 3000)
		parts = append(parts, header+body)
	}
	joined := strings.Join(parts, "\n\n")
	return truncateRunes(joined, 12000)
}

func safeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "(untitled)"
	}
	return title
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	return string([]rune(value)[:max])
}

func formatField(value any) string {
	formatted := strings.TrimSpace(fmt.Sprint(value))
	if formatted == "" || formatted == "<nil>" {
		return "not specified"
	}
	return formatted
}
