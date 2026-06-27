package lexers

import (
	"strings"

	. "github.com/alecthomas/chroma/v2" // nolint
)

// Go lexer.
var Go = Register(MustNewLexer(
	&Config{
		Name:      "Go",
		Aliases:   []string{"go", "golang"},
		Filenames: []string{"*.go"},
		MimeTypes: []string{"text/x-gosrc"},
	},
	goRules,
).SetAnalyser(func(text string) float32 {
	if strings.Contains(text, "fmt.") && strings.Contains(text, "package ") {
		return 0.5
	}
	if strings.Contains(text, "package ") {
		return 0.1
	}
	return 0.0
}))

func goRules() Rules {
	return Rules{
		"root": {
			{`\n`, TextWhitespace, nil},
			{`\s+`, TextWhitespace, nil},
			{`//[^\s][^\n\r]*`, CommentPreproc, nil},
			{`//\s+[^\n\r]*`, CommentSingle, nil},
			{`/(\\\n)?[*](.|\n)*?[*](\\\n)?/`, CommentMultiline, nil},
			{`(import|package)\b`, KeywordNamespace, nil},
			{`(var|func|struct|map|chan|type|interface|const)\b`, KeywordDeclaration, nil},
			{Words(``, `\b`, `break`, `default`, `select`, `case`, `defer`, `go`, `else`, `goto`, `switch`, `fallthrough`, `if`, `range`, `continue`, `for`, `return`), Keyword, nil},
			{`(true|false|iota|nil)\b`, KeywordConstant, nil},
			{Words(``, `\b(\()`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `int`, `int8`, `int16`, `int32`, `int64`, `float`, `float32`, `float64`, `complex64`, `complex128`, `byte`, `rune`, `string`, `bool`, `error`, `uintptr`, `print`, `println`, `panic`, `recover`, `close`, `complex`, `real`, `imag`, `len`, `cap`, `append`, `copy`, `delete`, `new`, `make`, `clear`, `min`, `max`), ByGroups(NameBuiltin, Punctuation), nil},
			{Words(``, `\b`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `int`, `int8`, `int16`, `int32`, `int64`, `float`, `float32`, `float64`, `complex64`, `complex128`, `byte`, `rune`, `string`, `bool`, `error`, `uintptr`, `any`), KeywordType, nil},
			{`\d+i`, LiteralNumber, nil},
			{`\d+\.\d*([Ee][-+]\d+)?i`, LiteralNumber, nil},
			{`\.\d+([Ee][-+]\d+)?i`, LiteralNumber, nil},
			{`\d+[Ee][-+]\d+i`, LiteralNumber, nil},
			{`\d+(\.\d+[eE][+\-]?\d+|\.\d*|[eE][+\-]?\d+)`, LiteralNumberFloat, nil},
			{`\.\d+([eE][+\-]?\d+)?`, LiteralNumberFloat, nil},
			{`0[0-7]+`, LiteralNumberOct, nil},
			{`0[xX][0-9a-fA-F_]+`, LiteralNumberHex, nil},
			{`0b[01_]+`, LiteralNumberBin, nil},
			{`(0|[1-9][0-9_]*)`, LiteralNumberInteger, nil},
			{`'(\\['"\\abfnrtv]|\\x[0-9a-fA-F]{2}|\\[0-7]{1,3}|\\u[0-9a-fA-F]{4}|\\U[0-9a-fA-F]{8}|[^\\])'`, LiteralStringChar, nil},
			{"(`)([^`]*)(`)", ByGroups(LiteralString, UsingLexer(TypeRemappingLexer(GoTextTemplate, TypeMapping{{Other, LiteralString, nil}})), LiteralString), nil},
			{`"(\\\\|\\"|[^"])*"`, LiteralString, nil},
			{`(<<=|>>=|<<|>>|<=|>=|&\^=|&\^|\+=|-=|\*=|/=|%=|&=|\|=|&&|\|\||<-|\+\+|--|==|!=|:=|\.\.\.|[+\-*/%&])`, Operator, nil},
			{`([a-zA-Z_]\w*)(\s*)(\()`, ByGroups(NameFunction, UsingSelf("root"), Punctuation), nil},
			{`[|^<>=!()\[\]{}.,;:~]`, Punctuation, nil},
			{`[^\W\d]\w*`, NameOther, nil},
		},
	}
}

var GoHTMLTemplate = Register(DelegatingLexer(HTML, MustNewXMLLexer(
	embedded,
	"embedded/go_template.xml",
).SetConfig(
	&Config{
		Name:    "Go HTML Template",
		Aliases: []string{"go-html-template"},
	},
)))

var GoTextTemplate = Register(MustNewXMLLexer(
	embedded,
	"embedded/go_template.xml",
).SetConfig(
	&Config{
		Name:    "Go Text Template",
		Aliases: []string{"go-text-template"},
	},
))
