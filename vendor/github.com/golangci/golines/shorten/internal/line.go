package internal

// LineLength gets the width of the provided line after tab expansion.
func LineLength(line string, tabLen int) int {
	length := 0

	for _, char := range line {
		if char == '\t' {
			length += tabLen
		} else {
			length++
		}
	}

	return length
}
