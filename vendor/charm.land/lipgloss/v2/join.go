package lipgloss

import (
	"math"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// JoinHorizontal is a utility function for horizontally joining two
// potentially multi-lined strings along a vertical axis. The first argument is
// the position, with 0 being all the way at the top and 1 being all the way
// at the bottom.
//
// If you just want to align to the top, center or bottom you may as well just
// use the helper constants Top, Center, and Bottom.
//
// Example:
//
//	blockB := "...\n...\n..."
//	blockA := "...\n...\n...\n...\n..."
//
//	// Join 20% from the top
//	str := lipgloss.JoinHorizontal(0.2, blockA, blockB)
//
//	// Join on the top edge
//	str := lipgloss.JoinHorizontal(lipgloss.Top, blockA, blockB)
func JoinHorizontal(pos Position, strs ...string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	var (
		// Groups of strings broken into multiple lines
		blocks = make([][]string, len(strs))

		// Max line widths for the above text blocks
		maxWidths = make([]int, len(strs))

		// Height of the tallest block
		maxHeight int
	)

	// Break text blocks into lines and get max widths for each text block
	for i, str := range strs {
		blocks[i], maxWidths[i] = getLines(str)
		if len(blocks[i]) > maxHeight {
			maxHeight = len(blocks[i])
		}
	}

	// Add extra lines to make each side the same height
	for i := range blocks {
		if len(blocks[i]) >= maxHeight {
			continue
		}

		extraLines := make([]string, maxHeight-len(blocks[i]))

		switch pos {
		case Top:
			blocks[i] = append(blocks[i], extraLines...)

		case Bottom:
			blocks[i] = append(extraLines, blocks[i]...)

		default: // Somewhere in the middle
			n := len(extraLines)
			split := int(math.Round(float64(n) * pos.value()))
			top := n - split
			bottom := n - top

			blocks[i] = append(extraLines[top:], blocks[i]...)
			blocks[i] = append(blocks[i], extraLines[bottom:]...)
		}
	}

	// Merge lines
	var b strings.Builder
	for i := range blocks[0] { // remember, all blocks have the same number of members now
		for j, block := range blocks {
			b.WriteString(block[i])

			// Also make lines the same length
			b.WriteString(strings.Repeat(" ", maxWidths[j]-ansi.StringWidth(block[i])))
		}
		if i < len(blocks[0])-1 {
			b.WriteRune('\n')
		}
	}

	return b.String()
}

// JoinVertical is a utility function for vertically joining two potentially
// multi-lined strings along a horizontal axis. The first argument is the
// position, with 0 being all the way to the left and 1 being all the way to
// the right.
//
// If you just want to align to the left, right or center you may as well just
// use the helper constants Left, Center, and Right.
//
// Example:
//
//	blockB := "...\n...\n..."
//	blockA := "...\n...\n...\n...\n..."
//
//	// Join 20% from the top
//	str := lipgloss.JoinVertical(0.2, blockA, blockB)
//
//	// Join on the right edge
//	str := lipgloss.JoinVertical(lipgloss.Right, blockA, blockB)
func JoinVertical(pos Position, strs ...string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	var (
		blocks   = make([][]string, len(strs))
		maxWidth int
	)

	for i := range strs {
		var w int
		blocks[i], w = getLines(strs[i])
		if w > maxWidth {
			maxWidth = w
		}
	}

	var b strings.Builder
	for i, block := range blocks {
		for j, line := range block {
			w := maxWidth - ansi.StringWidth(line)

			switch pos {
			case Left:
				b.WriteString(line)
				b.WriteString(strings.Repeat(" ", w))

			case Right:
				b.WriteString(strings.Repeat(" ", w))
				b.WriteString(line)

			default: // Somewhere in the middle
				if w < 1 {
					b.WriteString(line)
					break
				}

				split := int(math.Round(float64(w) * pos.value()))
				right := w - split
				left := w - right

				b.WriteString(strings.Repeat(" ", left))
				b.WriteString(line)
				b.WriteString(strings.Repeat(" ", right))
			}

			// Write a newline as long as we're not on the last line of the
			// last block.
			if !(i == len(blocks)-1 && j == len(block)-1) { //nolint:staticcheck
				b.WriteRune('\n')
			}
		}
	}

	return b.String()
}
