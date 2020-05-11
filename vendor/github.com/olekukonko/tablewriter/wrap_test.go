// Copyright 2014 Oleku Konko All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// This module is a Table Writer  API for the Go Programming Language.
// The protocols were written in pure Go and works on windows and unix systems

package tablewriter

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

var text = "The quick brown fox jumps over the lazy dog."

func TestWrap(t *testing.T) {
	exp := []string{
		"The", "quick", "brown", "fox",
		"jumps", "over", "the", "lazy", "dog."}

	got, _ := WrapString(text, 6)
	if len(exp) != len(got) {
		t.Fail()
	}
}

func TestWrapOneLine(t *testing.T) {
	exp := "The quick brown fox jumps over the lazy dog."
	words, _ := WrapString(text, 500)
	got := strings.Join(words, string(sp))
	if exp != got {
		t.Errorf("expected: %q, got: %q", exp, got)
	}
}

func TestUnicode(t *testing.T) {
	input := "Česká řeřicha"
	wordsUnicode, _ := WrapString(input, 13)
	// input contains 13 runes, so it fits on one line.
	if len(wordsUnicode) != 1 {
		t.Fail()
	}
}

func TestDisplayWidth(t *testing.T) {
	input := "Česká řeřicha"
	want := 13
	if runewidth.IsEastAsian() {
		want = 14
	}
	if n := DisplayWidth(input); n != want {
		t.Errorf("Wants: %d Got: %d", want, n)
	}
	input = "\033[43;30m" + input + "\033[00m"
	if n := DisplayWidth(input); n != want {
		t.Errorf("Wants: %d Got: %d", want, n)
	}
}
