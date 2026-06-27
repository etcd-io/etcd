package chroma

import (
	"fmt"
)

// An Emitter takes group matches and returns tokens.
type Emitter interface {
	// Emit tokens for the given regex groups.
	Emit(groups []string, state *LexerState) Iterator
}

// ValidatingEmitter is an Emitter that can validate against a compiled rule.
type ValidatingEmitter interface {
	Emitter
	ValidateEmitter(rule *CompiledRule) error
}

// SerialisableEmitter is an Emitter that can be serialised and deserialised to/from JSON.
type SerialisableEmitter interface {
	Emitter
	EmitterKind() string
}

// EmitterFunc is a function that is an Emitter.
type EmitterFunc func(groups []string, state *LexerState) Iterator

// Emit tokens for groups.
func (e EmitterFunc) Emit(groups []string, state *LexerState) Iterator {
	return e(groups, state)
}

type Emitters []Emitter

type byGroupsEmitter struct {
	Emitters
}

var _ ValidatingEmitter = (*byGroupsEmitter)(nil)

// ByGroups emits a token for each matching group in the rule's regex.
func ByGroups(emitters ...Emitter) Emitter {
	return &byGroupsEmitter{Emitters: emitters}
}

func (b *byGroupsEmitter) EmitterKind() string { return "bygroups" }

func (b *byGroupsEmitter) ValidateEmitter(rule *CompiledRule) error {
	if len(rule.Regexp.GetGroupNumbers())-1 != len(b.Emitters) {
		return fmt.Errorf("number of groups %d does not match number of emitters %d", len(rule.Regexp.GetGroupNumbers())-1, len(b.Emitters))
	}
	return nil
}

func (b *byGroupsEmitter) Emit(groups []string, state *LexerState) Iterator {
	iterators := make([]Iterator, 0, len(groups)-1)
	if len(b.Emitters) != len(groups)-1 {
		iterators = append(iterators, Error.Emit(groups, state))
		// panic(errors.Errorf("number of groups %q does not match number of emitters %v", groups, emitters))
	} else {
		for i, group := range groups[1:] {
			if b.Emitters[i] != nil {
				iterators = append(iterators, b.Emitters[i].Emit([]string{group}, state))
			}
		}
	}
	return Concaterator(iterators...)
}

// ByGroupNames emits a token for each named matching group in the rule's regex.
func ByGroupNames(emitters map[string]Emitter) Emitter {
	return EmitterFunc(func(groups []string, state *LexerState) Iterator {
		iterators := make([]Iterator, 0, len(state.NamedGroups)-1)
		if len(state.NamedGroups)-1 == 0 {
			if emitter, ok := emitters[`0`]; ok {
				iterators = append(iterators, emitter.Emit(groups, state))
			} else {
				iterators = append(iterators, Error.Emit(groups, state))
			}
		} else {
			ruleRegex := state.Rules[state.State][state.Rule].Regexp
			for i := 1; i < len(state.NamedGroups); i++ {
				groupName := ruleRegex.GroupNameFromNumber(i)
				group := state.NamedGroups[groupName]
				if emitter, ok := emitters[groupName]; ok {
					if emitter != nil {
						iterators = append(iterators, emitter.Emit([]string{group}, state))
					}
				} else {
					iterators = append(iterators, Error.Emit([]string{group}, state))
				}
			}
		}
		return Concaterator(iterators...)
	})
}

// UsingByGroup emits tokens for the matched groups in the regex using a
// sublexer. Used when lexing code blocks where the name of a sublexer is
// contained within the block, for example on a Markdown text block or SQL
// language block.
//
// An attempt to load the sublexer will be made using the captured value from
// the text of the matched sublexerNameGroup. If a sublexer matching the
// sublexerNameGroup is available, then tokens for the matched codeGroup will
// be emitted using the sublexer. Otherwise, if no sublexer is available, then
// tokens will be emitted from the passed emitter.
//
// Example:
//
//	var Markdown = internal.Register(MustNewLexer(
//		&Config{
//			Name:      "markdown",
//			Aliases:   []string{"md", "mkd"},
//			Filenames: []string{"*.md", "*.mkd", "*.markdown"},
//			MimeTypes: []string{"text/x-markdown"},
//		},
//		Rules{
//			"root": {
//				{"^(```)(\\w+)(\\n)([\\w\\W]*?)(^```$)",
//					UsingByGroup(
//						2, 4,
//						String, String, String, Text, String,
//					),
//					nil,
//				},
//			},
//		},
//	))
//
// See the lexers/markdown.go for the complete example.
//
// Note: panic's if the number of emitters does not equal the number of matched
// groups in the regex.
func UsingByGroup(sublexerNameGroup, codeGroup int, emitters ...Emitter) Emitter {
	return &usingByGroup{
		SublexerNameGroup: sublexerNameGroup,
		CodeGroup:         codeGroup,
		Emitters:          emitters,
	}
}

type usingByGroup struct {
	SublexerNameGroup int      `xml:"sublexer_name_group"`
	CodeGroup         int      `xml:"code_group"`
	Emitters          Emitters `xml:"emitters"`
}

func (u *usingByGroup) EmitterKind() string { return "usingbygroup" }
func (u *usingByGroup) Emit(groups []string, state *LexerState) Iterator {
	// bounds check
	if len(u.Emitters) != len(groups)-1 {
		panic("UsingByGroup expects number of emitters to be the same as len(groups)-1")
	}

	// grab sublexer
	sublexer := state.Registry.Get(groups[u.SublexerNameGroup])

	// build iterators
	iterators := make([]Iterator, len(groups)-1)
	for i, group := range groups[1:] {
		if i == u.CodeGroup-1 && sublexer != nil {
			var err error
			iterators[i], err = sublexer.Tokenise(nil, groups[u.CodeGroup])
			if err != nil {
				panic(err)
			}
		} else if u.Emitters[i] != nil {
			iterators[i] = u.Emitters[i].Emit([]string{group}, state)
		}
	}
	return Concaterator(iterators...)
}

// UsingLexer returns an Emitter that uses a given Lexer for parsing and emitting.
//
// This Emitter is not serialisable.
func UsingLexer(lexer Lexer) Emitter {
	return EmitterFunc(func(groups []string, _ *LexerState) Iterator {
		it, err := lexer.Tokenise(&TokeniseOptions{State: "root", Nested: true}, groups[0])
		if err != nil {
			panic(err)
		}
		return it
	})
}

type usingEmitter struct {
	Lexer string `xml:"lexer,attr"`
}

func (u *usingEmitter) EmitterKind() string { return "using" }

func (u *usingEmitter) Emit(groups []string, state *LexerState) Iterator {
	if state.Registry == nil {
		panic(fmt.Sprintf("no LexerRegistry available for Using(%q)", u.Lexer))
	}
	lexer := state.Registry.Get(u.Lexer)
	if lexer == nil {
		panic(fmt.Sprintf("no such lexer %q", u.Lexer))
	}
	it, err := lexer.Tokenise(&TokeniseOptions{State: "root", Nested: true}, groups[0])
	if err != nil {
		panic(err)
	}
	return it
}

// Using returns an Emitter that uses a given Lexer reference for parsing and emitting.
//
// The referenced lexer must be stored in the same LexerRegistry.
func Using(lexer string) Emitter {
	return &usingEmitter{Lexer: lexer}
}

type usingSelfEmitter struct {
	State string `xml:"state,attr"`
}

func (u *usingSelfEmitter) EmitterKind() string { return "usingself" }

func (u *usingSelfEmitter) Emit(groups []string, state *LexerState) Iterator {
	it, err := state.Lexer.Tokenise(&TokeniseOptions{State: u.State, Nested: true}, groups[0])
	if err != nil {
		panic(err)
	}
	return it
}

// UsingSelf is like Using, but uses the current Lexer.
func UsingSelf(stateName string) Emitter {
	return &usingSelfEmitter{stateName}
}
