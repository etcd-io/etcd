package pkg

import "regexp"

func fn() {
	regexp.Compile("")          // MATCH "does not contain any meta characters"
	regexp.Compile("abc")       // MATCH "does not contain any meta characters"
	regexp.Compile("abc\\.def") // MATCH "does not contain any meta characters"
	regexp.Compile("abc.def")
	regexp.Compile("(abc)")
}
