package lexers

import (
	"strings"

	. "github.com/alecthomas/chroma/v2" // nolint
)

// phtml lexer is PHP in HTML.
var _ = Register(DelegatingLexer(HTML, MustNewLexer(
	&Config{
		Name:            "PHTML",
		Aliases:         []string{"phtml"},
		Filenames:       []string{"*.phtml", "*.php", "*.php[345]", "*.inc"},
		MimeTypes:       []string{"application/x-php", "application/x-httpd-php", "application/x-httpd-php3", "application/x-httpd-php4", "application/x-httpd-php5", "text/x-php"},
		DotAll:          true,
		CaseInsensitive: true,
		EnsureNL:        true,
		Priority:        2,
	},
	func() Rules {
		return Get("PHP").(*RegexLexer).MustRules().
			Rename("root", "php").
			Merge(Rules{
				"root": {
					{`<\?(php)?`, CommentPreproc, Push("php")},
					{`[^<]+`, Other, nil},
					{`<`, Other, nil},
				},
			})
	},
).SetAnalyser(func(text string) float32 {
	if strings.Contains(text, "<?php") {
		return 0.5
	}
	return 0.0
})))
