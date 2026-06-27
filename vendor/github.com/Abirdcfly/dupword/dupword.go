// MIT License
//
// Copyright (c) 2022 Abirdcfly
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package dupword defines an Analyzer that checks those duplicate words in the source code.
package dupword

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	Name = "dupword"
	Doc  = `checks for duplicate words in the source code (usually miswritten)

This analyzer checks miswritten duplicate words in comments or package doc or string declaration`
	Message       = "Duplicate words (%s) found"
	CommentPrefix = `//`
)

type keywords []string

func (a keywords) String() string {
	return strings.Join(a, ",")
}

func (a *keywords) Set(w string) error {
	if len(w) != 0 {
		*a = append(*a, strings.Split(w, ",")...)
	}

	return nil
}

type ignore map[string]bool

func (a ignore) String() string {
	var t []string

	for k := range a {
		t = append(t, k)
	}

	return strings.Join(t, ",")
}

func (a ignore) Set(w string) error {
	for _, k := range strings.Split(w, ",") {
		a[k] = true
	}

	return nil
}

func NewAnalyzer() *analysis.Analyzer {
	analyzer := &analyzer{
		ignoreWords: map[string]bool{},
	}

	a := &analysis.Analyzer{
		Name:             Name,
		Doc:              Doc,
		Requires:         []*analysis.Analyzer{inspect.Analyzer},
		Run:              analyzer.run,
		RunDespiteErrors: true,
	}

	a.Flags.Init(Name, flag.ExitOnError)
	a.Flags.Var(&analyzer.keywords, "keyword", "keywords for detecting duplicate words")
	a.Flags.Var(&analyzer.ignoreWords, "ignore", "ignore words")
	a.Flags.BoolVar(&analyzer.commentsOnly, "comments-only", false, "check only comments, skip strings")
	a.Flags.Var(version{}, "V", "print version and exit")

	return a
}

type analyzer struct {
	keywords     keywords
	ignoreWords  ignore
	commentsOnly bool
}

func (a *analyzer) run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		a.fixDuplicateWordInComment(pass, file)
	}
	if a.commentsOnly {
		return nil, nil
	}
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.BasicLit)(nil),
	}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		if lit, ok := n.(*ast.BasicLit); ok {
			a.fixDuplicateWordInString(pass, lit)
		}
	})
	return nil, nil
}

func (a *analyzer) fixDuplicateWordInComment(pass *analysis.Pass, f *ast.File) {
	isTestFile := strings.HasSuffix(pass.Fset.File(f.FileStart).Name(), "_test.go")
	for _, cg := range f.Comments {
		// avoid checking example outputs for duplicate words
		if isTestFile && isExampleOutputStart(cg.List[0].Text) {
			continue
		}
		var preLine *ast.Comment
		for _, c := range cg.List {
			update, keyword, find := a.Check(c.Text)
			if find {
				pass.Report(analysis.Diagnostic{Pos: c.Slash, End: c.End(), Message: fmt.Sprintf(Message, keyword), SuggestedFixes: []analysis.SuggestedFix{{
					Message: "Update",
					TextEdits: []analysis.TextEdit{{
						Pos:     c.Slash,
						End:     c.End(),
						NewText: []byte(update),
					}},
				}}})
			}
			if preLine != nil {
				fields := strings.Fields(preLine.Text)
				if len(fields) < 1 {
					continue
				}
				preLineContent := fields[len(fields)-1] + "\n"
				thisLineContent := c.Text
				if find {
					thisLineContent = update
				}
				before, after, _ := strings.Cut(thisLineContent, CommentPrefix)
				update, keyword, find := a.Check(preLineContent + after)
				if find {
					var suggestedFixes []analysis.SuggestedFix
					if strings.Contains(update, preLineContent) {
						update = before + CommentPrefix + strings.TrimPrefix(update, preLineContent)
						suggestedFixes = []analysis.SuggestedFix{{
							Message: "Update",
							TextEdits: []analysis.TextEdit{{
								Pos:     c.Slash,
								End:     c.End(),
								NewText: []byte(update),
							}},
						}}
					}
					pass.Report(analysis.Diagnostic{Pos: c.Slash, End: c.End(), Message: fmt.Sprintf(Message, keyword), SuggestedFixes: suggestedFixes})
				}
			}
			preLine = c
		}
	}
}

func (a *analyzer) fixDuplicateWordInString(pass *analysis.Pass, lit *ast.BasicLit) {
	if lit.Kind != token.STRING {
		return
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		fmt.Printf("lit.Value:%v, err: %v\n", lit.Value, err)
		// fall back to default
		value = lit.Value
	}
	quote := value != lit.Value
	update, keyword, find := a.Check(value)
	if quote {
		update = strconv.Quote(update)
	}
	if find {
		pass.Report(analysis.Diagnostic{Pos: lit.Pos(), End: lit.End(), Message: fmt.Sprintf(Message, keyword), SuggestedFixes: []analysis.SuggestedFix{{
			Message: "Update",
			TextEdits: []analysis.TextEdit{{
				Pos:     lit.Pos(),
				End:     lit.End(),
				NewText: []byte(update),
			}},
		}}})
	}
}

// CheckOneKey use to check there is a defined duplicate word in a string.
// `raw` is the checked line. key is the keyword to check. empty means just check duplicate word.
func (a *analyzer) checkOneKey(raw, key string) (new string, findWord string, find bool) {
	if key == "" {
		has := false
		fields := strings.Fields(raw)
		for i := range fields {
			if i == len(fields)-1 {
				break
			}
			if fields[i] == fields[i+1] {
				has = true
			}
		}
		if !has {
			return
		}
	} else {
		if x := strings.Split(raw, key); len(x) < 2 {
			return
		}
	}

	findWordMap := make(map[string]bool, 4)
	newLine := strings.Builder{}
	wordStart, spaceStart := 0, 0
	curWord, preWord := "", ""
	lastSpace := ""
	var lastRune int32
	for i, r := range raw {
		if !unicode.IsSpace(r) && unicode.IsSpace(lastRune) {
			// word start position
			/*
				                                             i
				                                             |
					hello[ spaceA ]the[ spaceB ]the[ spaceC ]word
				                   ^            ^
				                   |            curWord: the
				                   preWord: the
			*/
			symbol := raw[spaceStart:i]
			if ((key != "" && curWord == key) || key == "") && curWord == preWord && curWord != "" {
				if !a.excludeWords(cutTrailingCommas(curWord)) {
					find = true
					findWordMap[curWord] = true
					newLine.WriteString(lastSpace)
					symbol = ""
				}
			} else {
				newLine.WriteString(lastSpace)
				newLine.WriteString(curWord)
			}
			lastSpace = symbol
			preWord = curWord
			wordStart = i
		} else if unicode.IsSpace(r) && !unicode.IsSpace(lastRune) {
			// space start position
			spaceStart = i
			curWord = raw[wordStart:i]
		} else if i == len(raw)-1 {
			// last position
			word := raw[wordStart:]
			if ((key != "" && word == key) || key == "") && word == preWord {
				if !a.excludeWords(cutTrailingCommas(word)) {
					find = true
					findWordMap[word] = true
				}
			} else {
				newLine.WriteString(lastSpace)
				newLine.WriteString(word)
			}
		}
		lastRune = r
	}
	if find {
		new = newLine.String()
		findWordSlice := make([]string, len(findWordMap))
		i := 0
		for k := range findWordMap {
			findWordSlice[i] = k
			i++
		}
		sort.Strings(findWordSlice)
		findWord = strings.Join(findWordSlice, ",")
	}
	return
}

func (a *analyzer) Check(raw string) (update string, keyword string, find bool) {
	for _, key := range a.keywords {
		updateOne, _, findOne := a.checkOneKey(raw, key)
		if findOne {
			raw = updateOne
			find = findOne
			update = updateOne
			if keyword == "" {
				keyword = key
			} else {
				keyword = keyword + "," + key
			}
		}
	}
	if len(a.keywords) == 0 {
		return a.checkOneKey(raw, "")
	}
	return
}

// ExcludeWords determines whether duplicate words should be reported,
//
//	e.g. %s, </div> should not be reported.
func (a *analyzer) excludeWords(word string) (exclude bool) {
	firstRune, _ := utf8.DecodeRuneInString(word)
	if unicode.IsDigit(firstRune) {
		return true
	}
	if unicode.IsPunct(firstRune) {
		return true
	}
	if unicode.IsSymbol(firstRune) {
		return true
	}
	if _, exist := a.ignoreWords[word]; exist {
		return true
	}
	return false
}

func isExampleOutputStart(comment string) bool {
	return strings.HasPrefix(comment, "// Output:") ||
		strings.HasPrefix(comment, "// output:") ||
		strings.HasPrefix(comment, "// Unordered output:") ||
		strings.HasPrefix(comment, "// unordered output:")
}

// cutTrailingCommas is used to remove trailing commas of words.
// The excludeWords are provided as comma-separated list, so it is
// impossible to ignore "[word], [word]," matches otherwise
func cutTrailingCommas(s string) string {
	result, _ := strings.CutSuffix(s, ",")
	return result
}
