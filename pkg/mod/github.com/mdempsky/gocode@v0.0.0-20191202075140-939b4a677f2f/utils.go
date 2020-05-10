package main

import (
	"os"
	"unicode/utf8"
)

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return true
}

func runeToByteOffset(s []byte, offset_c int) (offset_b int) {
	for offset_b = 0; offset_c > 0 && offset_b < len(s); offset_b++ {
		if utf8.RuneStart(s[offset_b]) {
			offset_c--
		}
	}
	return offset_b
}
