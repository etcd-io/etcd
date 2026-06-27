package lexers

import (
	. "github.com/alecthomas/chroma/v2" // nolint
)

// Svelte lexer.
var Svelte = Register(DelegatingLexer(HTML, MustNewLexer(
	&Config{
		Name:      "Svelte",
		Aliases:   []string{"svelte"},
		Filenames: []string{"*.svelte"},
		MimeTypes: []string{"application/x-svelte"},
		DotAll:    true,
	},
	svelteRules,
)))

func svelteRules() Rules {
	return Rules{
		"root": {
			// Let HTML handle the comments, including comments containing script and style tags
			{`<!--`, Other, Push("comment")},
			{
				// Highlight script and style tags based on lang attribute
				// and allow attributes besides lang
				`(<\s*(?:script|style).*?lang\s*=\s*['"])` +
					`(.+?)(['"].*?>)` +
					`(.+?)` +
					`(<\s*/\s*(?:script|style)\s*>)`,
				UsingByGroup(2, 4, Other, Other, Other, Other, Other),
				nil,
			},
			{
				// Make sure `{` is not inside script or style tags
				`(?<!<\s*(?:script|style)(?:(?!(?:script|style)\s*>).)*?)` +
					`{` +
					`(?!(?:(?!<\s*(?:script|style)).)*?(?:script|style)\s*>)`,
				Punctuation,
				Push("templates"),
			},
			// on:submit|preventDefault
			{`(?<=\s+on:\w+(?:\|\w+)*)\|(?=\w+)`, Operator, nil},
			{`.+?`, Other, nil},
		},
		"comment": {
			{`-->`, Other, Pop(1)},
			{`.+?`, Other, nil},
		},
		"templates": {
			{`}`, Punctuation, Pop(1)},
			// Let TypeScript handle strings and the curly braces inside them
			{`(?<!(?<!\\)\\)(['"` + "`])" + `.*?(?<!(?<!\\)\\)\1`, Using("TypeScript"), nil},
			// If there is another opening curly brace push to templates again
			{"{", Punctuation, Push("templates")},
			{`@(debug|html)\b`, Keyword, nil},
			{
				`(#await)(\s+)(\w+)(\s+)(then|catch)(\s+)(\w+)`,
				ByGroups(Keyword, Text, Using("TypeScript"), Text,
					Keyword, Text, Using("TypeScript"),
				),
				nil,
			},
			{`(#|/)(await|each|if|key)\b`, Keyword, nil},
			{`(:else)(\s+)(if)?\b`, ByGroups(Keyword, Text, Keyword), nil},
			{`:(catch|then)\b`, Keyword, nil},
			{`[^{}]+`, Using("TypeScript"), nil},
		},
	}
}
