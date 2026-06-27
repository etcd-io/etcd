package lexers

import (
	"strings"

	. "github.com/alecthomas/chroma/v2" // nolint
)

// ERB lexer is Ruby embedded in HTML.
var ERB = Register(DelegatingLexer(HTML, MustNewXMLLexer(
	embedded,
	"embedded/erb.xml",
).SetConfig(
	&Config{
		Name:      "ERB",
		Aliases:   []string{"erb", "html+erb", "html+ruby", "rhtml"},
		Filenames: []string{"*.erb", "*.html.erb", "*.xml.erb", "*.rhtml"},
		MimeTypes: []string{"application/x-ruby-templating"},
		DotAll:    true,
	},
).SetAnalyser(func(text string) float32 {
	if strings.Contains(text, "<%=") && strings.Contains(text, "%>") {
		return 0.4
	}
	if strings.Contains(text, "<%") {
		return 0.1
	}
	return 0.0
})))
