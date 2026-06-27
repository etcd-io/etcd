package lexers

import (
	. "github.com/alecthomas/chroma/v2" // nolint
)

// Matcher token stub for docs, or
// Named matcher: @name, or
// Path matcher: /foo, or
// Wildcard path matcher: *
// nolint: gosec
var caddyfileMatcherTokenRegexp = `(\[\<matcher\>\]|@[^\s]+|/[^\s]+|\*)`

// Comment at start of line, or
// Comment preceded by whitespace
var caddyfileCommentRegexp = `(^|\s+)#.*\n`

// caddyfileCommon are the rules common to both of the lexer variants
func caddyfileCommonRules() Rules {
	return Rules{
		"site_block_common": {
			Include("site_body"),
			// Any other directive
			{`[^\s#]+`, Keyword, Push("directive")},
			Include("base"),
		},
		"site_body": {
			// Import keyword
			{`\b(import|invoke)\b( [^\s#]+)`, ByGroups(Keyword, Text), Push("subdirective")},
			// Matcher definition
			{`@[^\s]+(?=\s)`, NameDecorator, Push("matcher")},
			// Matcher token stub for docs
			{`\[\<matcher\>\]`, NameDecorator, Push("matcher")},
			// These cannot have matchers but may have things that look like
			// matchers in their arguments, so we just parse as a subdirective.
			{`\b(try_files|tls|log|bind)\b`, Keyword, Push("subdirective")},
			// These are special, they can nest more directives
			{`\b(handle_errors|handle_path|handle_response|replace_status|handle|route)\b`, Keyword, Push("nested_directive")},
			// uri directive has special syntax
			{`\b(uri)\b`, Keyword, Push("uri_directive")},
		},
		"matcher": {
			{`\{`, Punctuation, Push("block")},
			// Not can be one-liner
			{`not`, Keyword, Push("deep_not_matcher")},
			// Heredoc for CEL expression
			Include("heredoc"),
			// Backtick for CEL expression
			{"`", StringBacktick, Push("backticks")},
			// Any other same-line matcher
			{`[^\s#]+`, Keyword, Push("arguments")},
			// Terminators
			{`\s*\n`, Text, Pop(1)},
			{`\}`, Punctuation, Pop(1)},
			Include("base"),
		},
		"block": {
			{`\}`, Punctuation, Pop(2)},
			// Using double quotes doesn't stop at spaces
			{`"`, StringDouble, Push("double_quotes")},
			// Using backticks doesn't stop at spaces
			{"`", StringBacktick, Push("backticks")},
			// Not can be one-liner
			{`not`, Keyword, Push("not_matcher")},
			// Directives & matcher definitions
			Include("site_body"),
			// Any directive
			{`[^\s#]+`, Keyword, Push("subdirective")},
			Include("base"),
		},
		"nested_block": {
			{`\}`, Punctuation, Pop(2)},
			// Using double quotes doesn't stop at spaces
			{`"`, StringDouble, Push("double_quotes")},
			// Using backticks doesn't stop at spaces
			{"`", StringBacktick, Push("backticks")},
			// Not can be one-liner
			{`not`, Keyword, Push("not_matcher")},
			// Directives & matcher definitions
			Include("site_body"),
			// Any other subdirective
			{`[^\s#]+`, Keyword, Push("directive")},
			Include("base"),
		},
		"not_matcher": {
			{`\}`, Punctuation, Pop(2)},
			{`\{(?=\s)`, Punctuation, Push("block")},
			{`[^\s#]+`, Keyword, Push("arguments")},
			{`\s+`, Text, nil},
		},
		"deep_not_matcher": {
			{`\}`, Punctuation, Pop(2)},
			{`\{(?=\s)`, Punctuation, Push("block")},
			{`[^\s#]+`, Keyword, Push("deep_subdirective")},
			{`\s+`, Text, nil},
		},
		"directive": {
			{`\{(?=\s)`, Punctuation, Push("block")},
			{caddyfileMatcherTokenRegexp, NameDecorator, Push("arguments")},
			{caddyfileCommentRegexp, CommentSingle, Pop(1)},
			{`\s*\n`, Text, Pop(1)},
			Include("base"),
		},
		"nested_directive": {
			{`\{(?=\s)`, Punctuation, Push("nested_block")},
			{caddyfileMatcherTokenRegexp, NameDecorator, Push("nested_arguments")},
			{caddyfileCommentRegexp, CommentSingle, Pop(1)},
			{`\s*\n`, Text, Pop(1)},
			Include("base"),
		},
		"subdirective": {
			{`\{(?=\s)`, Punctuation, Push("block")},
			{caddyfileCommentRegexp, CommentSingle, Pop(1)},
			{`\s*\n`, Text, Pop(1)},
			Include("base"),
		},
		"arguments": {
			{`\{(?=\s)`, Punctuation, Push("block")},
			{caddyfileCommentRegexp, CommentSingle, Pop(2)},
			{`\\\n`, Text, nil}, // Skip escaped newlines
			{`\s*\n`, Text, Pop(2)},
			Include("base"),
		},
		"nested_arguments": {
			{`\{(?=\s)`, Punctuation, Push("nested_block")},
			{caddyfileCommentRegexp, CommentSingle, Pop(2)},
			{`\\\n`, Text, nil}, // Skip escaped newlines
			{`\s*\n`, Text, Pop(2)},
			Include("base"),
		},
		"deep_subdirective": {
			{`\{(?=\s)`, Punctuation, Push("block")},
			{caddyfileCommentRegexp, CommentSingle, Pop(3)},
			{`\s*\n`, Text, Pop(3)},
			Include("base"),
		},
		"uri_directive": {
			{`\{(?=\s)`, Punctuation, Push("block")},
			{caddyfileMatcherTokenRegexp, NameDecorator, nil},
			{`(strip_prefix|strip_suffix|replace|path_regexp)`, NameConstant, Push("arguments")},
			{caddyfileCommentRegexp, CommentSingle, Pop(1)},
			{`\s*\n`, Text, Pop(1)},
			Include("base"),
		},
		"double_quotes": {
			Include("placeholder"),
			{`\\"`, StringDouble, nil},
			{`[^"]`, StringDouble, nil},
			{`"`, StringDouble, Pop(1)},
		},
		"backticks": {
			Include("placeholder"),
			{"\\\\`", StringBacktick, nil},
			{"[^`]", StringBacktick, nil},
			{"`", StringBacktick, Pop(1)},
		},
		"optional": {
			// Docs syntax for showing optional parts with [ ]
			{`\[`, Punctuation, Push("optional")},
			Include("name_constants"),
			{`\|`, Punctuation, nil},
			{`[^\[\]\|]+`, String, nil},
			{`\]`, Punctuation, Pop(1)},
		},
		"heredoc": {
			{`(<<([a-zA-Z0-9_-]+))(\n(.*|\n)*)(\s*)(\2)`, ByGroups(StringHeredoc, nil, String, String, String, StringHeredoc), nil},
		},
		"name_constants": {
			{`\b(most_recently_modified|largest_size|smallest_size|first_exist|internal|disable_redirects|ignore_loaded_certs|disable_certs|private_ranges|first|last|before|after|on|off)\b(\||(?=\]|\s|$))`, ByGroups(NameConstant, Punctuation), nil},
		},
		"placeholder": {
			// Placeholder with dots, colon for default value, brackets for args[0:]
			{`\{[\w+.\[\]\:\$-]+\}`, StringEscape, nil},
			// Handle opening brackets with no matching closing one
			{`\{[^\}\s]*\b`, String, nil},
		},
		"base": {
			{caddyfileCommentRegexp, CommentSingle, nil},
			{`\[\<matcher\>\]`, NameDecorator, nil},
			Include("name_constants"),
			Include("heredoc"),
			{`(https?://)?([a-z0-9.-]+)(:)([0-9]+)([^\s]*)`, ByGroups(Name, Name, Punctuation, NumberInteger, Name), nil},
			{`\[`, Punctuation, Push("optional")},
			{"`", StringBacktick, Push("backticks")},
			{`"`, StringDouble, Push("double_quotes")},
			Include("placeholder"),
			{`[a-z-]+/[a-z-+]+`, String, nil},
			{`[0-9]+([smhdk]|ns|us|µs|ms)?\b`, NumberInteger, nil},
			{`[^\s\n#\{]+`, String, nil},
			{`/[^\s#]*`, Name, nil},
			{`\s+`, Text, nil},
		},
	}
}

// Caddyfile lexer.
var Caddyfile = Register(MustNewLexer(
	&Config{
		Name:      "Caddyfile",
		Aliases:   []string{"caddyfile", "caddy"},
		Filenames: []string{"Caddyfile*"},
		MimeTypes: []string{},
	},
	caddyfileRules,
))

func caddyfileRules() Rules {
	return Rules{
		"root": {
			{caddyfileCommentRegexp, CommentSingle, nil},
			// Global options block
			{`^\s*(\{)\s*$`, ByGroups(Punctuation), Push("globals")},
			// Top level import
			{`(import)(\s+)([^\s]+)`, ByGroups(Keyword, Text, NameVariableMagic), nil},
			// Snippets
			{`(&?\([^\s#]+\))(\s*)(\{)`, ByGroups(NameVariableAnonymous, Text, Punctuation), Push("snippet")},
			// Site label
			{`[^#{(\s,]+`, GenericHeading, Push("label")},
			// Site label with placeholder
			{`\{[\w+.\[\]\:\$-]+\}`, StringEscape, Push("label")},
			{`\s+`, Text, nil},
		},
		"globals": {
			{`\}`, Punctuation, Pop(1)},
			// Global options are parsed as subdirectives (no matcher)
			{`[^\s#]+`, Keyword, Push("subdirective")},
			Include("base"),
		},
		"snippet": {
			{`\}`, Punctuation, Pop(1)},
			Include("site_body"),
			// Any other directive
			{`[^\s#]+`, Keyword, Push("directive")},
			Include("base"),
		},
		"label": {
			// Allow multiple labels, comma separated, newlines after
			// a comma means another label is coming
			{`,\s*\n?`, Text, nil},
			{` `, Text, nil},
			// Site label with placeholder
			Include("placeholder"),
			// Site label
			{`[^#{(\s,]+`, GenericHeading, nil},
			// Comment after non-block label (hack because comments end in \n)
			{`#.*\n`, CommentSingle, Push("site_block")},
			// Note: if \n, we'll never pop out of the site_block, it's valid
			{`\{(?=\s)|\n`, Punctuation, Push("site_block")},
		},
		"site_block": {
			{`\}`, Punctuation, Pop(2)},
			Include("site_block_common"),
		},
	}.Merge(caddyfileCommonRules())
}

// Caddyfile directive-only lexer.
var CaddyfileDirectives = Register(MustNewLexer(
	&Config{
		Name:      "Caddyfile Directives",
		Aliases:   []string{"caddyfile-directives", "caddyfile-d", "caddy-d"},
		Filenames: []string{},
		MimeTypes: []string{},
	},
	caddyfileDirectivesRules,
))

func caddyfileDirectivesRules() Rules {
	return Rules{
		// Same as "site_block" in Caddyfile
		"root": {
			Include("site_block_common"),
		},
	}.Merge(caddyfileCommonRules())
}
