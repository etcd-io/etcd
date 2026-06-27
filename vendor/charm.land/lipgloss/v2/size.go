package lipgloss

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Width returns the cell width of characters in the string. ANSI sequences are
// ignored and characters wider than one cell (such as Chinese characters and
// emojis) are appropriately measured.
//
// You should use this instead of len(string) or len([]rune(string) as neither
// will give you accurate results.
func Width(str string) (width int) {
	for l := range strings.SplitSeq(str, "\n") {
		w := ansi.StringWidth(l)
		if w > width {
			width = w
		}
	}

	return width
}

// Height returns height of a string in cells. This is done simply by
// counting \n characters. If your output has \r\n, that sequence will be
// replaced with a \n in [Style.Render].
func Height(str string) int {
	return strings.Count(str, "\n") + 1
}

// Size returns the width and height of the string in cells. ANSI sequences are
// ignored and characters wider than one cell (such as Chinese characters and
// emojis) are appropriately measured.
func Size(str string) (width, height int) {
	width = Width(str)
	height = Height(str)
	return width, height
}
