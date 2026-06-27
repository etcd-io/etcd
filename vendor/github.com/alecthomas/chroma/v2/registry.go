package chroma

import (
	"path/filepath"
	"sort"
	"strings"
)

var (
	ignoredSuffixes = [...]string{
		// Editor backups
		"~", ".bak", ".old", ".orig",
		// Debian and derivatives apt/dpkg/ucf backups
		".dpkg-dist", ".dpkg-old", ".ucf-dist", ".ucf-new", ".ucf-old",
		// Red Hat and derivatives rpm backups
		".rpmnew", ".rpmorig", ".rpmsave",
		// Build system input/template files
		".in",
	}
)

// LexerRegistry is a registry of Lexers.
type LexerRegistry struct {
	Lexers  Lexers
	byName  map[string]Lexer
	byAlias map[string]Lexer
}

// NewLexerRegistry creates a new LexerRegistry of Lexers.
func NewLexerRegistry() *LexerRegistry {
	return &LexerRegistry{
		byName:  map[string]Lexer{},
		byAlias: map[string]Lexer{},
	}
}

// Names of all lexers, optionally including aliases.
func (l *LexerRegistry) Names(withAliases bool) []string {
	out := []string{}
	for _, lexer := range l.Lexers {
		config := lexer.Config()
		out = append(out, config.Name)
		if withAliases {
			out = append(out, config.Aliases...)
		}
	}
	sort.Strings(out)
	return out
}

// Aliases of all the lexers, and skip those lexers who do not have any aliases,
// or show their name instead
func (l *LexerRegistry) Aliases(skipWithoutAliases bool) []string {
	out := []string{}
	for _, lexer := range l.Lexers {
		config := lexer.Config()
		if len(config.Aliases) == 0 {
			if skipWithoutAliases {
				continue
			}
			out = append(out, config.Name)
		}
		out = append(out, config.Aliases...)
	}
	sort.Strings(out)
	return out
}

// Get a Lexer by name, alias or file extension.
func (l *LexerRegistry) Get(name string) Lexer {
	if lexer := l.byName[name]; lexer != nil {
		return lexer
	}
	if lexer := l.byAlias[name]; lexer != nil {
		return lexer
	}
	if lexer := l.byName[strings.ToLower(name)]; lexer != nil {
		return lexer
	}
	if lexer := l.byAlias[strings.ToLower(name)]; lexer != nil {
		return lexer
	}

	candidates := PrioritisedLexers{}
	// Try file extension.
	if lexer := l.Match("filename." + name); lexer != nil {
		candidates = append(candidates, lexer)
	}
	// Try exact filename.
	if lexer := l.Match(name); lexer != nil {
		candidates = append(candidates, lexer)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Sort(candidates)
	return candidates[0]
}

// MatchMimeType attempts to find a lexer for the given MIME type.
func (l *LexerRegistry) MatchMimeType(mimeType string) Lexer {
	matched := PrioritisedLexers{}
	for _, l := range l.Lexers {
		for _, lmt := range l.Config().MimeTypes {
			if mimeType == lmt {
				matched = append(matched, l)
			}
		}
	}
	if len(matched) != 0 {
		sort.Sort(matched)
		return matched[0]
	}
	return nil
}

// Match returns the first lexer matching filename.
//
// Note that this iterates over all file patterns in all lexers, so is not fast.
func (l *LexerRegistry) Match(filename string) Lexer {
	filename = filepath.Base(filename)
	matched := PrioritisedLexers{}
	// First, try primary filename matches.
	for _, lexer := range l.Lexers {
		config := lexer.Config()
		for _, glob := range config.Filenames {
			ok, err := filepath.Match(glob, filename)
			if err != nil { // nolint
				panic(err)
			} else if ok {
				matched = append(matched, lexer)
			} else {
				for _, suf := range &ignoredSuffixes {
					ok, err := filepath.Match(glob+suf, filename)
					if err != nil {
						panic(err)
					} else if ok {
						matched = append(matched, lexer)
						break
					}
				}
			}
		}
	}
	if len(matched) > 0 {
		sort.Sort(matched)
		return matched[0]
	}
	matched = nil
	// Next, try filename aliases.
	for _, lexer := range l.Lexers {
		config := lexer.Config()
		for _, glob := range config.AliasFilenames {
			ok, err := filepath.Match(glob, filename)
			if err != nil { // nolint
				panic(err)
			} else if ok {
				matched = append(matched, lexer)
			} else {
				for _, suf := range &ignoredSuffixes {
					ok, err := filepath.Match(glob+suf, filename)
					if err != nil {
						panic(err)
					} else if ok {
						matched = append(matched, lexer)
						break
					}
				}
			}
		}
	}
	if len(matched) > 0 {
		sort.Sort(matched)
		return matched[0]
	}
	return nil
}

// Analyse text content and return the "best" lexer..
func (l *LexerRegistry) Analyse(text string) Lexer {
	var picked Lexer
	highest := float32(0.0)
	for _, lexer := range l.Lexers {
		if analyser, ok := lexer.(Analyser); ok {
			weight := analyser.AnalyseText(text)
			if weight > highest {
				picked = lexer
				highest = weight
			}
		}
	}
	return picked
}

// Register a Lexer with the LexerRegistry. If the lexer is already registered
// it will be replaced.
func (l *LexerRegistry) Register(lexer Lexer) Lexer {
	lexer.SetRegistry(l)
	config := lexer.Config()

	l.byName[config.Name] = lexer
	l.byName[strings.ToLower(config.Name)] = lexer

	for _, alias := range config.Aliases {
		l.byAlias[alias] = lexer
		l.byAlias[strings.ToLower(alias)] = lexer
	}

	l.Lexers = add(l.Lexers, lexer)

	return lexer
}

// add adds a lexer to a slice of lexers if it doesn't already exist, or if found will replace it.
func add(lexers Lexers, lexer Lexer) Lexers {
	for i, val := range lexers {
		if val == nil {
			continue
		}

		if val.Config().Name == lexer.Config().Name {
			lexers[i] = lexer
			return lexers
		}
	}

	return append(lexers, lexer)
}
