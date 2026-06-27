// Copyright 2014 Oleku Konko All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// This module is a Table Writer API for the Go Programming Language.
// The protocols were written in pure Go and works on windows and unix systems

package twwarp

import (
	"math"
	"strings"
	"unicode"

	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/olekukonko/tablewriter/pkg/twwidth"
)

const (
	nl = "\n"
	sp = " "
)

const defaultPenalty = 1e5

func SplitWords(s string) []string {
	words := make([]string, 0, len(s)/5)
	var wordBegin int
	wordPending := false
	for i, c := range s {
		if unicode.IsSpace(c) {
			if wordPending {
				words = append(words, s[wordBegin:i])
				wordPending = false
			}
			continue
		}
		if !wordPending {
			wordBegin = i
			wordPending = true
		}
	}
	if wordPending {
		words = append(words, s[wordBegin:])
	}
	return words
}

// WrapString wraps s into a paragraph of lines of length lim, with minimal
// raggedness.
func WrapString(s string, lim int) ([]string, int) {
	if s == sp {
		return []string{sp}, lim
	}
	words := SplitWords(s)
	if len(words) == 0 {
		return []string{""}, lim
	}
	var lines []string
	max := 0
	for _, v := range words {
		max = twwidth.Width(v)
		if max > lim {
			lim = max
		}
	}
	for _, line := range WrapWords(words, 1, lim, defaultPenalty) {
		lines = append(lines, strings.Join(line, sp))
	}
	return lines, lim
}

// WrapStringWithSpaces wraps a string into lines of a specified display width while preserving
// leading and trailing spaces. It splits the input string into words, condenses internal multiple
// spaces to a single space, and wraps the content to fit within the given width limit, measured
// using Unicode-aware display width. The function is used in the logging library to format log
// messages for consistent output. It returns the wrapped lines as a slice of strings and the
// adjusted width limit, which may increase if a single word exceeds the input limit. Thread-safe
// as it does not modify shared state.
func WrapStringWithSpaces(s string, lim int) ([]string, int) {
	if len(s) == 0 {
		return []string{""}, lim
	}
	if strings.TrimSpace(s) == "" { // All spaces
		if twwidth.Width(s) <= lim {
			return []string{s}, twwidth.Width(s)
		}
		// For very long all-space strings, "wrap" by truncating to the limit.
		if lim > 0 {
			substring, _ := stringToDisplayWidth(s, lim)
			return []string{substring}, lim
		}
		return []string{""}, lim
	}

	var leadingSpaces, trailingSpaces, coreContent string
	firstNonSpace := strings.IndexFunc(s, func(r rune) bool { return !unicode.IsSpace(r) })
	leadingSpaces = s[:firstNonSpace]
	lastNonSpace := strings.LastIndexFunc(s, func(r rune) bool { return !unicode.IsSpace(r) })
	trailingSpaces = s[lastNonSpace+1:]
	coreContent = s[firstNonSpace : lastNonSpace+1]

	if coreContent == "" {
		return []string{leadingSpaces + trailingSpaces}, lim
	}

	words := SplitWords(coreContent)
	if len(words) == 0 {
		return []string{leadingSpaces + trailingSpaces}, lim
	}

	var lines []string
	currentLim := lim

	maxCoreWordWidth := 0
	for _, v := range words {
		w := twwidth.Width(v)
		if w > maxCoreWordWidth {
			maxCoreWordWidth = w
		}
	}

	if maxCoreWordWidth > currentLim {
		currentLim = maxCoreWordWidth
	}

	wrappedWordLines := WrapWords(words, 1, currentLim, defaultPenalty)

	for i, lineWords := range wrappedWordLines {
		joinedLine := strings.Join(lineWords, sp)
		finalLine := leadingSpaces + joinedLine
		if i == len(wrappedWordLines)-1 { // Last line
			finalLine += trailingSpaces
		}
		lines = append(lines, finalLine)
	}
	return lines, currentLim
}

// stringToDisplayWidth returns a substring of s that has a display width
// as close as possible to, but not exceeding, targetWidth.
// It returns the substring and its actual display width.
func stringToDisplayWidth(s string, targetWidth int) (substring string, actualWidth int) {
	if targetWidth <= 0 {
		return "", 0
	}

	var currentWidth int
	var endIndex int // Tracks the byte index in the original string

	g := graphemes.FromString(s)
	for g.Next() {
		grapheme := g.Value()
		graphemeWidth := twwidth.Width(grapheme)

		if currentWidth+graphemeWidth > targetWidth {
			break
		}

		currentWidth += graphemeWidth
		endIndex = g.End()
	}
	return s[:endIndex], currentWidth
}

// WrapWords is the low-level line-breaking algorithm, useful if you need more
// control over the details of the text wrapping process. For most uses,
// WrapString will be sufficient and more convenient.
//
// WrapWords splits a list of words into lines with minimal "raggedness",
// treating each rune as one unit, accounting for spc units between adjacent
// words on each line, and attempting to limit lines to lim units. Raggedness
// is the total error over all lines, where error is the square of the
// difference of the length of the line and lim. Too-long lines (which only
// happen when a single word is longer than lim units) have pen penalty units
// added to the error.
func WrapWords(words []string, spc, lim, pen int) [][]string {
	n := len(words)
	if n == 0 {
		return nil
	}
	lengths := make([]int, n)
	for i := 0; i < n; i++ {
		lengths[i] = twwidth.Width(words[i])
	}
	nbrk := make([]int, n)
	cost := make([]int, n)
	for i := range cost {
		cost[i] = math.MaxInt32
	}
	remainderLen := lengths[n-1] // Uses updated lengths
	for i := n - 1; i >= 0; i-- {
		if i < n-1 {
			remainderLen += spc + lengths[i]
		}
		if remainderLen <= lim {
			cost[i] = 0
			nbrk[i] = n
			continue
		}
		phraseLen := lengths[i]
		for j := i + 1; j < n; j++ {
			if j > i+1 {
				phraseLen += spc + lengths[j-1]
			}
			d := lim - phraseLen
			c := d*d + cost[j]
			if phraseLen > lim {
				c += pen // too-long lines get a worse penalty
			}
			if c < cost[i] {
				cost[i] = c
				nbrk[i] = j
			}
		}
	}
	var lines [][]string
	i := 0
	for i < n {
		lines = append(lines, words[i:nbrk[i]])
		i = nbrk[i]
	}
	return lines
}

// getLines decomposes a multiline string into a slice of strings.
func getLines(s string) []string {
	return strings.Split(s, nl)
}
