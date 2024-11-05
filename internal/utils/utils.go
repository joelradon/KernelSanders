// internal/utils/utils.go

package utils

import (
	"strings"
	"time"
)

// SummarizeToLength trims the text to the specified maximum length.
// If the text exceeds the maxLength, it truncates and appends '...'.
func SummarizeToLength(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// ExtractKeywords extracts keywords from the given text by removing common stopwords.
// This is a simple implementation and can be enhanced with more sophisticated NLP techniques.
func ExtractKeywords(text string) string {
	stopwords := map[string]struct{}{
		"the": {}, "is": {}, "at": {}, "which": {}, "on": {}, "and": {},
		"a": {}, "an": {}, "in": {}, "with": {}, "to": {}, "from": {},
		"by": {}, "for": {}, "of": {}, "or": {}, "as": {}, "that": {},
		"this": {}, "it": {}, "be": {}, "are": {}, "was": {}, "were": {},
		"been": {}, "being": {}, "have": {}, "has": {}, "had": {}, "do": {},
		"does": {}, "did": {}, "but": {}, "if": {}, "while": {}, "can": {},
		"could": {}, "should": {}, "would": {}, "may": {}, "might": {},
		"must": {}, "will": {}, "shall": {},
	}

	words := strings.Fields(strings.ToLower(text))
	var keywords []string
	for _, word := range words {
		cleanWord := strings.Trim(word, ".,!?\"'()[]{};:")
		if _, isStopword := stopwords[cleanWord]; !isStopword && len(cleanWord) > 2 {
			keywords = append(keywords, cleanWord)
		}
	}

	return strings.Join(keywords, ";")
}

// FormatTimeUTC formats a time.Time object to a string in UTC format.
func FormatTimeUTC(t time.Time) string {
	return t.UTC().Format(time.RFC1123)
}

// FormatTimeEDT formats a time.Time object to a string in EDT format.
func FormatTimeEDT(t time.Time) string {
	edtZone := time.FixedZone("EDT", -4*3600) // EDT is UTC-4
	return t.In(edtZone).Format(time.RFC1123)
}
