package lexers

import (
	. "github.com/alecthomas/chroma/v2" // nolint
)

// Markless lexer.
var Markless = Register(MustNewLexer(
	&Config{
		Name:      "Markless",
		Aliases:   []string{"mess"},
		Filenames: []string{"*.mess", "*.markless"},
		MimeTypes: []string{"text/x-markless"},
	},
	marklessRules,
))

func marklessRules() Rules {
	return Rules{
		"root": {
			Include("block"),
		},
		// Block directives
		"block": {
			Include("header"),
			Include("ordered-list"),
			Include("unordered-list"),
			Include("code-block"),
			Include("blockquote"),
			Include("blockquote-header"),
			Include("align"),
			Include("comment"),
			Include("instruction"),
			Include("embed"),
			Include("footnote"),
			Include("horizontal-rule"),
			Include("paragraph"),
		},
		"header": {
			{`(# )(.*)$`, ByGroups(Keyword, GenericHeading), Push("inline")},
			{`(##+)(.*)$`, ByGroups(Keyword, GenericSubheading), Push("inline")},
		},
		"ordered-list": {
			{`([0-9]+\.)`, Keyword, nil},
		},
		"unordered-list": {
			{`(- )`, Keyword, nil},
		},
		"code-block": {
			{`(::+)( *)(\w*)([^\n]*)(\n)([\w\W]*?)(^\1$)`, UsingByGroup(3, 6, Keyword, TextWhitespace, NameFunction, String, TextWhitespace, Text, Keyword), nil},
		},
		"blockquote": {
			{`(\| )(.*)$`, ByGroups(Keyword, GenericInserted), nil},
		},
		"blockquote-header": {
			{`(~ )([^|\n]+)(\| )(.*?\n)`, ByGroups(Keyword, NameEntity, Keyword, GenericInserted), Push("inline-blockquote")},
			{`(~ )(.*)$`, ByGroups(Keyword, NameEntity), nil},
		},
		"inline-blockquote": {
			{`^(   +)(\| )(.*$)`, ByGroups(TextWhitespace, Keyword, GenericInserted), nil},
			Default(Pop(1)),
		},
		"align": {
			{`(\|\|)|(\|<)|(\|>)|(><)`, Keyword, nil},
		},
		"comment": {
			{`(;[; ]).*?$`, CommentSingle, nil},
		},
		"instruction": {
			{`(! )([^ ]+)(.+?)$`, ByGroups(Keyword, NameFunction, NameVariable), nil},
		},
		"embed": {
			{`(\[ )([^ ]+)( )([^,]+)`, ByGroups(Keyword, NameFunction, TextWhitespace, String), Push("embed-options")},
		},
		"embed-options": {
			{`\\.`, Text, nil},
			{`,`, Punctuation, nil},
			{`\]?$`, Keyword, Pop(1)},
			// Generic key or key/value pair
			{`( *)([^, \]]+)([^,\]]+)?`, ByGroups(TextWhitespace, NameFunction, String), nil},
			{`.`, Text, nil},
		},
		"footnote": {
			{`(\[)([0-9]+)(\])`, ByGroups(Keyword, NameVariable, Keyword), Push("inline")},
		},
		"horizontal-rule": {
			{`(==+)$`, LiteralOther, nil},
		},
		"paragraph": {
			{` *`, TextWhitespace, Push("inline")},
		},
		// Inline directives
		"inline": {
			Include("escapes"),
			Include("dashes"),
			Include("newline"),
			Include("italic"),
			Include("underline"),
			Include("bold"),
			Include("strikethrough"),
			Include("code"),
			Include("compound"),
			Include("footnote-reference"),
			Include("subtext"),
			Include("subtext"),
			Include("url"),
			{`.`, Text, nil},
			{`\n`, TextWhitespace, Pop(1)},
		},
		"escapes": {
			{`\\.`, Text, nil},
		},
		"dashes": {
			{`-{2,3}`, TextPunctuation, nil},
		},
		"newline": {
			{`-/-`, TextWhitespace, nil},
		},
		"italic": {
			{`(//)(.*?)(\1)`, ByGroups(Keyword, GenericEmph, Keyword), nil},
		},
		"underline": {
			{`(__)(.*?)(\1)`, ByGroups(Keyword, GenericUnderline, Keyword), nil},
		},
		"bold": {
			{`(\*\*)(.*?)(\1)`, ByGroups(Keyword, GenericStrong, Keyword), nil},
		},
		"strikethrough": {
			{`(<-)(.*?)(->)`, ByGroups(Keyword, GenericDeleted, Keyword), nil},
		},
		"code": {
			{"(``+)(.*?)(\\1)", ByGroups(Keyword, LiteralStringBacktick, Keyword), nil},
		},
		"compound": {
			{`(''+)(.*?)(''\()`, ByGroups(Keyword, UsingSelf("inline"), Keyword), Push("compound-options")},
		},
		"compound-options": {
			{`\\.`, Text, nil},
			{`,`, Punctuation, nil},
			{`\)`, Keyword, Pop(1)},
			// Hex Color
			{` *#[0-9A-Fa-f]{3,6} *`, LiteralNumberHex, nil},
			// Named Color
			{` *(indian-red|light-coral|salmon|dark-salmon|light-salmon|crimson|red|firebrick|dark-red|pink|light-pink|hot-pink|deep-pink|medium-violet-red|pale-violet-red|coral|tomato|orange-red|dark-orange|orange|gold|yellow|light-yellow|lemon-chiffon|light-goldenrod-yellow|papayawhip|moccasin|peachpuff|pale-goldenrod|khaki|dark-khaki|lavender|thistle|plum|violet|orchid|fuchsia|magenta|medium-orchid|medium-purple|rebecca-purple|blue-violet|dark-violet|dark-orchid|dark-magenta|purple|indigo|slate-blue|dark-slate-blue|medium-slate-blue|green-yellow|chartreuse|lawn-green|lime|lime-green|pale-green|light-green|medium-spring-green|spring-green|medium-sea-green|sea-green|forest-green|green|dark-green|yellow-green|olive-drab|olive|dark-olive-green|medium-aquamarine|dark-sea-green|light-sea-green|dark-cyan|teal|aqua|cyan|light-cyan|pale-turquoise|aquamarine|turquoise|medium-turquoise|dark-turquoise|cadet-blue|steel-blue|light-steel-blue|powder-blue|light-blue|sky-blue|light-sky-blue|deep-sky-blue|dodger-blue|cornflower-blue|royal-blue|blue|medium-blue|dark-blue|navy|midnight-blue|cornsilk|blanched-almond|bisque|navajo-white|wheat|burlywood|tan|rosy-brown|sandy-brown|goldenrod|dark-goldenrod|peru|chocolate|saddle-brown|sienna|brown|maroon|white|snow|honeydew|mintcream|azure|alice-blue|ghost-white|white-smoke|seashell|beige|oldlace|floral-white|ivory|antique-white|linen|lavenderblush|mistyrose|gainsboro|light-gray|silver|dark-gray|gray|dim-gray|light-slate-gray|slate-gray|dark-slate-gray) *`, LiteralOther, nil},
			// Named size
			{` *(microscopic|tiny|small|normal|big|large|huge|gigantic) *`, NameTag, nil},
			// Options
			{` *(bold|italic|underline|strikethrough|subtext|supertext|spoiler) *`, NameBuiltin, nil},
			// URL. Note the missing ) and , in the match.
			{` *\w[-\w+.]*://[\w$\-_.+!*'(&/:;=?@z%#\\]+ *`, String, nil},
			// Generic key or key/value pair
			{`( *)([^, )]+)( [^,)]+)?`, ByGroups(TextWhitespace, NameFunction, String), nil},
			{`.`, Text, nil},
		},
		"footnote-reference": {
			{`(\[)([0-9]+)(\])`, ByGroups(Keyword, NameVariable, Keyword), nil},
		},
		"subtext": {
			{`(v\()(.*?)(\))`, ByGroups(Keyword, UsingSelf("inline"), Keyword), nil},
		},
		"supertext": {
			{`(\^\()(.*?)(\))`, ByGroups(Keyword, UsingSelf("inline"), Keyword), nil},
		},
		"url": {
			{`\w[-\w+.]*://[\w\$\-_.+!*'()&,/:;=?@z%#\\]+`, String, nil},
		},
	}
}
