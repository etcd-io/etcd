package chroma

import (
	"fmt"
	"strings"
)

var (
	defaultOptions = &TokeniseOptions{
		State:    "root",
		EnsureLF: true,
	}
)

// Config for a lexer.
type Config struct {
	// Name of the lexer.
	Name string `xml:"name,omitempty"`

	// Shortcuts for the lexer
	Aliases []string `xml:"alias,omitempty"`

	// File name globs
	Filenames []string `xml:"filename,omitempty"`

	// Secondary file name globs
	AliasFilenames []string `xml:"alias_filename,omitempty"`

	// MIME types
	MimeTypes []string `xml:"mime_type,omitempty"`

	// Regex matching is case-insensitive.
	CaseInsensitive bool `xml:"case_insensitive,omitempty"`

	// Regex matches all characters.
	DotAll bool `xml:"dot_all,omitempty"`

	// Regex does not match across lines ($ matches EOL).
	//
	// Defaults to multiline.
	NotMultiline bool `xml:"not_multiline,omitempty"`

	// Don't strip leading and trailing newlines from the input.
	// DontStripNL bool

	// Strip all leading and trailing whitespace from the input
	// StripAll bool

	// Make sure that the input ends with a newline. This
	// is required for some lexers that consume input linewise.
	EnsureNL bool `xml:"ensure_nl,omitempty"`

	// If given and greater than 0, expand tabs in the input.
	// TabSize int

	// Priority of lexer.
	//
	// If this is 0 it will be treated as a default of 1.
	Priority float32 `xml:"priority,omitempty"`

	// Analyse is a list of regexes to match against the input.
	//
	// If a match is found, the score is returned if single attribute is set to true,
	// otherwise the sum of all the score of matching patterns will be
	// used as the final score.
	Analyse *AnalyseConfig `xml:"analyse,omitempty"`
}

// AnalyseConfig defines the list of regexes analysers.
type AnalyseConfig struct {
	Regexes []RegexConfig `xml:"regex,omitempty"`
	// If true, the first matching score is returned.
	First bool `xml:"first,attr"`
}

// RegexConfig defines a single regex pattern and its score in case of match.
type RegexConfig struct {
	Pattern string  `xml:"pattern,attr"`
	Score   float32 `xml:"score,attr"`
}

// Token output to formatter.
type Token struct {
	Type  TokenType `json:"type"`
	Value string    `json:"value"`
}

func (t *Token) String() string   { return t.Value }
func (t *Token) GoString() string { return fmt.Sprintf("&Token{%s, %q}", t.Type, t.Value) }

// Clone returns a clone of the Token.
func (t *Token) Clone() Token {
	return *t
}

// EOF is returned by lexers at the end of input.
var EOF Token

// TokeniseOptions contains options for tokenisers.
type TokeniseOptions struct {
	// State to start tokenisation in. Defaults to "root".
	State string
	// Nested tokenisation.
	Nested bool

	// If true, all EOLs are converted into LF
	// by replacing CRLF and CR
	EnsureLF bool
}

// A Lexer for tokenising source code.
type Lexer interface {
	// Config describing the features of the Lexer.
	Config() *Config
	// Tokenise returns an Iterator over tokens in text.
	Tokenise(options *TokeniseOptions, text string) (Iterator, error)
	// SetRegistry sets the registry this Lexer is associated with.
	//
	// The registry should be used by the Lexer if it needs to look up other
	// lexers.
	SetRegistry(registry *LexerRegistry) Lexer
	// SetAnalyser sets a function the Lexer should use for scoring how
	// likely a fragment of text is to match this lexer, between 0.0 and 1.0.
	// A value of 1 indicates high confidence.
	//
	// Lexers may ignore this if they implement their own analysers.
	SetAnalyser(analyser func(text string) float32) Lexer
	// AnalyseText scores how likely a fragment of text is to match
	// this lexer, between 0.0 and 1.0. A value of 1 indicates high confidence.
	AnalyseText(text string) float32
}

// Trace is the trace of a tokenisation process.
type Trace struct {
	Lexer   string  `json:"lexer"`
	State   string  `json:"state"`
	Rule    int     `json:"rule"`
	Pattern string  `json:"pattern"`
	Pos     int     `json:"pos"`
	Length  int     `json:"length"`
	Elapsed float64 `json:"elapsedMs"` // Elapsed time spent matching for this rule.
}

// TracingLexer is a Lexer that can trace its tokenisation process.
type TracingLexer interface {
	Lexer
	SetTracing(enable bool)
}

// Lexers is a slice of lexers sortable by name.
type Lexers []Lexer

func (l Lexers) Len() int      { return len(l) }
func (l Lexers) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l Lexers) Less(i, j int) bool {
	return strings.ToLower(l[i].Config().Name) < strings.ToLower(l[j].Config().Name)
}

// PrioritisedLexers is a slice of lexers sortable by priority.
type PrioritisedLexers []Lexer

func (l PrioritisedLexers) Len() int      { return len(l) }
func (l PrioritisedLexers) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l PrioritisedLexers) Less(i, j int) bool {
	ip := l[i].Config().Priority
	if ip == 0 {
		ip = 1
	}
	jp := l[j].Config().Priority
	if jp == 0 {
		jp = 1
	}
	return ip > jp
}

// Analyser determines how appropriate this lexer is for the given text.
type Analyser interface {
	AnalyseText(text string) float32
}
