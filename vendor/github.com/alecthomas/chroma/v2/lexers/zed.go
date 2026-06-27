package lexers

import (
	"strings"
)

// Zed lexer.
func init() { // nolint: gochecknoinits
	Get("Zed").SetAnalyser(func(text string) float32 {
		if strings.Contains(text, "definition ") && strings.Contains(text, "relation ") && strings.Contains(text, "permission ") {
			return 0.9
		}
		if strings.Contains(text, "definition ") {
			return 0.5
		}
		if strings.Contains(text, "relation ") {
			return 0.5
		}
		if strings.Contains(text, "permission ") {
			return 0.25
		}
		return 0.0
	})
}
