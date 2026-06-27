package misspell

import (
	"strings"
)

// WordCase is an enum of various word casing styles.
type WordCase int

// Various WordCase types... likely to be not correct.
const (
	CaseUnknown WordCase = iota
	CaseLower
	CaseUpper
	CaseTitle
)

// CaseStyle returns what case style a word is in.
func CaseStyle(word string) WordCase {
	upperCount := 0
	lowerCount := 0

	// this iterates over RUNES not BYTES
	for i := range len(word) {
		ch := word[i]
		switch {
		case ch >= 'a' && ch <= 'z':
			lowerCount++
		case ch >= 'A' && ch <= 'Z':
			upperCount++
		}
	}

	switch {
	case upperCount != 0 && lowerCount == 0:
		return CaseUpper
	case upperCount == 0 && lowerCount != 0:
		return CaseLower
	case upperCount == 1 && lowerCount > 0 && word[0] >= 'A' && word[0] <= 'Z':
		return CaseTitle
	}

	return CaseUnknown
}

// CaseVariations returns:
// If AllUpper or First-Letter-Only is upper-cased: add the all upper case version.
// If AllLower, add the original, the title and  upper-case forms.
// If Mixed, return the original, and the all  upper-case form.
func CaseVariations(word string, style WordCase) []string {
	switch style {
	case CaseLower:
		return []string{word, strings.ToUpper(word[0:1]) + word[1:], strings.ToUpper(word)}
	case CaseUpper:
		return []string{strings.ToUpper(word)}
	default:
		return []string{word, strings.ToUpper(word)}
	}
}
