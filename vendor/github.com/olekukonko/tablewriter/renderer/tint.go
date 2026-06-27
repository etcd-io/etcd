package renderer

import "github.com/fatih/color"

// Colors is a slice of color attributes for use with fatih/color, such as color.FgWhite or color.Bold.
type Colors []color.Attribute

// Tint defines foreground and background color settings for table elements, with optional per-column overrides.
type Tint struct {
	FG      Colors // Foreground color attributes
	BG      Colors // Background color attributes
	Columns []Tint // Per-column color settings
}

// Apply applies the Tint's foreground and background colors to the given text, returning the text unchanged if no colors are set.
func (t Tint) Apply(text string) string {
	if len(t.FG) == 0 && len(t.BG) == 0 {
		return text
	}
	// Combine foreground and background colors
	combinedColors := append(t.FG, t.BG...)
	// Create a color function and apply it to the text
	c := color.New(combinedColors...).SprintFunc()
	return c(text)
}
