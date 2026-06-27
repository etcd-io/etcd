package analyzer

import (
	"go/ast"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

var errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func exprImplementsError(pass *analysis.Pass, e ast.Expr) bool {
	return typeImplementsError(pass.TypesInfo.TypeOf(e))
}

func typeImplementsError(t types.Type) bool {
	return t != nil && types.Implements(t, errorIface)
}

func isValidErrorTypeName(s string) bool {
	if isInitialism(s) {
		return true
	}

	words := split(s)
	wordsCnt := wordsCount(words)

	if wordsCnt["error"] != 1 {
		return false
	}
	return words[len(words)-1] == "error"
}

func isValidErrorArrayTypeName(s string) bool {
	if isInitialism(s) {
		return true
	}

	words := split(s)
	wordsCnt := wordsCount(words)

	if wordsCnt["errors"] != 1 && wordsCnt["error"] != 1 {
		return false
	}

	lastWord := words[len(words)-1]
	return lastWord == "errors" || lastWord == "error"
}

func isValidErrorVarName(s string) bool {
	if isInitialism(s) {
		return true
	}

	words := split(s)
	wordsCnt := wordsCount(words)

	if wordsCnt["err"] != 1 {
		return false
	}
	return words[0] == "err"
}

func isInitialism(s string) bool {
	return strings.ToLower(s) == s || strings.ToUpper(s) == s
}

func split(s string) []string {
	var words []string
	ss := []rune(s)

	var b strings.Builder
	b.WriteRune(ss[0])

	for _, r := range ss[1:] {
		if unicode.IsUpper(r) {
			words = append(words, strings.ToLower(b.String()))
			b.Reset()
		}
		b.WriteRune(r)
	}

	words = append(words, strings.ToLower(b.String()))
	return words
}

func wordsCount(w []string) map[string]int {
	result := make(map[string]int, len(w))
	for _, ww := range w {
		result[ww]++
	}
	return result
}
