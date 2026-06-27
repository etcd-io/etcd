package chroma

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// A Rule is the fundamental matching unit of the Regex lexer state machine.
type Rule struct {
	Pattern string
	Type    Emitter
	Mutator Mutator
}

// Words creates a regex that matches any of the given literal words.
func Words(prefix, suffix string, words ...string) string {
	sort.Slice(words, func(i, j int) bool {
		return len(words[j]) < len(words[i])
	})
	for i, word := range words {
		words[i] = regexp.QuoteMeta(word)
	}
	return prefix + `(` + strings.Join(words, `|`) + `)` + suffix
}

// Tokenise text using lexer, returning tokens as a slice.
func Tokenise(lexer Lexer, options *TokeniseOptions, text string) ([]Token, error) {
	var out []Token
	it, err := lexer.Tokenise(options, text)
	if err != nil {
		return nil, err
	}
	for t := it(); t != EOF; t = it() {
		out = append(out, t)
	}
	return out, nil
}

// Rules maps from state to a sequence of Rules.
type Rules map[string][]Rule

// Rename clones rules then a rule.
func (r Rules) Rename(oldRule, newRule string) Rules {
	r = r.Clone()
	r[newRule] = r[oldRule]
	delete(r, oldRule)
	return r
}

// Clone returns a clone of the Rules.
func (r Rules) Clone() Rules {
	out := map[string][]Rule{}
	for key, rules := range r {
		out[key] = make([]Rule, len(rules))
		copy(out[key], rules)
	}
	return out
}

// Merge creates a clone of "r" then merges "rules" into the clone.
func (r Rules) Merge(rules Rules) Rules {
	out := r.Clone()
	for k, v := range rules.Clone() {
		out[k] = v
	}
	return out
}

// MustNewLexer creates a new Lexer with deferred rules generation or panics.
func MustNewLexer(config *Config, rules func() Rules) *RegexLexer {
	lexer, err := NewLexer(config, rules)
	if err != nil {
		panic(err)
	}
	return lexer
}

// NewLexer creates a new regex-based Lexer.
//
// "rules" is a state machine transition map. Each key is a state. Values are sets of rules
// that match input, optionally modify lexer state, and output tokens.
func NewLexer(config *Config, rulesFunc func() Rules) (*RegexLexer, error) {
	if config == nil {
		config = &Config{}
	}
	for _, glob := range append(config.Filenames, config.AliasFilenames...) {
		_, err := filepath.Match(glob, "")
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a valid glob: %w", config.Name, glob, err)
		}
	}
	r := &RegexLexer{
		config:         config,
		fetchRulesFunc: func() (Rules, error) { return rulesFunc(), nil },
	}
	// One-off code to generate XML lexers in the Chroma source tree.
	// var nameCleanRe = regexp.MustCompile(`[^-+A-Za-z0-9_]`)
	// name := strings.ToLower(nameCleanRe.ReplaceAllString(config.Name, "_"))
	// data, err := Marshal(r)
	// if err != nil {
	// 	if errors.Is(err, ErrNotSerialisable) {
	// 		fmt.Fprintf(os.Stderr, "warning: %q: %s\n", name, err)
	// 		return r, nil
	// 	}
	// 	return nil, err
	// }
	// _, file, _, ok := runtime.Caller(2)
	// if !ok {
	// 	panic("??")
	// }
	// fmt.Println(file)
	// if strings.Contains(file, "/lexers/") {
	// 	dir := filepath.Join(filepath.Dir(file), "embedded")
	// 	err = os.MkdirAll(dir, 0700)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	filename := filepath.Join(dir, name) + ".xml"
	// 	fmt.Println(filename)
	// 	err = ioutil.WriteFile(filename, data, 0600)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	return r, nil
}

// Trace enables debug tracing.
//
// Deprecated: Use SetTracing instead.
func (r *RegexLexer) Trace(trace bool) *RegexLexer {
	r.trace = trace
	return r
}

// SetTracing enables debug tracing.
//
// This complies with the [TracingLexer] interface.
func (r *RegexLexer) SetTracing(trace bool) {
	r.trace = trace
}

// A CompiledRule is a Rule with a pre-compiled regex.
//
// Note that regular expressions are lazily compiled on first use of the lexer.
type CompiledRule struct {
	Rule
	Regexp *regexp2.Regexp
	flags  string
}

// CompiledRules is a map of rule name to sequence of compiled rules in that rule.
type CompiledRules map[string][]*CompiledRule

// LexerState contains the state for a single lex.
type LexerState struct {
	Lexer    *RegexLexer
	Registry *LexerRegistry
	Text     []rune
	Pos      int
	Rules    CompiledRules
	Stack    []string
	State    string
	Rule     int
	// Group matches.
	Groups []string
	// Named Group matches.
	NamedGroups map[string]string
	// Custum context for mutators.
	MutatorContext map[interface{}]interface{}
	iteratorStack  []Iterator
	options        *TokeniseOptions
	newlineAdded   bool
}

// Set mutator context.
func (l *LexerState) Set(key interface{}, value interface{}) {
	l.MutatorContext[key] = value
}

// Get mutator context.
func (l *LexerState) Get(key interface{}) interface{} {
	return l.MutatorContext[key]
}

// Iterator returns the next Token from the lexer.
func (l *LexerState) Iterator() Token { // nolint: gocognit
	trace := json.NewEncoder(os.Stderr)
	end := len(l.Text)
	if l.newlineAdded {
		end--
	}
	for l.Pos < end && len(l.Stack) > 0 {
		// Exhaust the iterator stack, if any.
		for len(l.iteratorStack) > 0 {
			n := len(l.iteratorStack) - 1
			t := l.iteratorStack[n]()
			if t.Type == Ignore {
				continue
			}
			if t == EOF {
				l.iteratorStack = l.iteratorStack[:n]
				continue
			}
			return t
		}

		l.State = l.Stack[len(l.Stack)-1]
		selectedRule, ok := l.Rules[l.State]
		if !ok {
			panic("unknown state " + l.State)
		}
		var start time.Time
		if l.Lexer.trace {
			start = time.Now()
		}
		ruleIndex, rule, groups, namedGroups := matchRules(l.Text, l.Pos, selectedRule)
		if l.Lexer.trace {
			var length int
			if groups != nil {
				length = len(groups[0])
			} else {
				length = -1
			}
			_ = trace.Encode(Trace{ //nolint
				Lexer:   l.Lexer.config.Name,
				State:   l.State,
				Rule:    ruleIndex,
				Pattern: rule.Pattern,
				Pos:     l.Pos,
				Length:  length,
				Elapsed: float64(time.Since(start)) / float64(time.Millisecond),
			})
			// fmt.Fprintf(os.Stderr, "%s: pos=%d, text=%q, elapsed=%s\n", l.State, l.Pos, string(l.Text[l.Pos:]), time.Since(start))
		}
		// No match.
		if groups == nil {
			// From Pygments :\
			//
			// If the RegexLexer encounters a newline that is flagged as an error token, the stack is
			// emptied and the lexer continues scanning in the 'root' state. This can help producing
			// error-tolerant highlighting for erroneous input, e.g. when a single-line string is not
			// closed.
			if l.Text[l.Pos] == '\n' && l.State != l.options.State {
				l.Stack = []string{l.options.State}
				continue
			}
			l.Pos++
			return Token{Error, string(l.Text[l.Pos-1 : l.Pos])}
		}
		l.Rule = ruleIndex
		l.Groups = groups
		l.NamedGroups = namedGroups
		l.Pos += utf8.RuneCountInString(groups[0])
		if rule.Mutator != nil {
			if err := rule.Mutator.Mutate(l); err != nil {
				panic(err)
			}
		}
		if rule.Type != nil {
			l.iteratorStack = append(l.iteratorStack, rule.Type.Emit(l.Groups, l))
		}
	}
	// Exhaust the IteratorStack, if any.
	// Duplicate code, but eh.
	for len(l.iteratorStack) > 0 {
		n := len(l.iteratorStack) - 1
		t := l.iteratorStack[n]()
		if t.Type == Ignore {
			continue
		}
		if t == EOF {
			l.iteratorStack = l.iteratorStack[:n]
			continue
		}
		return t
	}

	// If we get to here and we still have text, return it as an error.
	if l.Pos != len(l.Text) && len(l.Stack) == 0 {
		value := string(l.Text[l.Pos:])
		l.Pos = len(l.Text)
		return Token{Type: Error, Value: value}
	}
	return EOF
}

// RegexLexer is the default lexer implementation used in Chroma.
type RegexLexer struct {
	registry *LexerRegistry // The LexerRegistry this Lexer is associated with, if any.
	config   *Config
	analyser func(text string) float32
	trace    bool

	mu             sync.Mutex
	compiled       bool
	rawRules       Rules
	rules          map[string][]*CompiledRule
	fetchRulesFunc func() (Rules, error)
	compileOnce    sync.Once
	compileError   error
}

func (r *RegexLexer) String() string {
	return r.config.Name
}

// Rules in the Lexer.
func (r *RegexLexer) Rules() (Rules, error) {
	if err := r.needRules(); err != nil {
		return nil, err
	}
	return r.rawRules, nil
}

// SetRegistry the lexer will use to lookup other lexers if necessary.
func (r *RegexLexer) SetRegistry(registry *LexerRegistry) Lexer {
	r.registry = registry
	return r
}

// SetAnalyser sets the analyser function used to perform content inspection.
func (r *RegexLexer) SetAnalyser(analyser func(text string) float32) Lexer {
	r.analyser = analyser
	return r
}

// AnalyseText scores how likely a fragment of text is to match this lexer, between 0.0 and 1.0.
func (r *RegexLexer) AnalyseText(text string) float32 {
	if r.analyser != nil {
		return r.analyser(text)
	}
	return 0
}

// SetConfig replaces the Config for this Lexer.
func (r *RegexLexer) SetConfig(config *Config) *RegexLexer {
	r.config = config
	return r
}

// Config returns the Config for this Lexer.
func (r *RegexLexer) Config() *Config {
	return r.config
}

// Regex compilation is deferred until the lexer is used. This is to avoid significant init() time costs.
func (r *RegexLexer) maybeCompile() (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.compiled {
		return nil
	}
	for state, rules := range r.rules {
		for i, rule := range rules {
			if rule.Regexp == nil {
				pattern := "(?:" + rule.Pattern + ")"
				if rule.flags != "" {
					pattern = "(?" + rule.flags + ")" + pattern
				}
				pattern = `\G` + pattern
				rule.Regexp, err = regexp2.Compile(pattern, 0)
				if err != nil {
					return fmt.Errorf("failed to compile rule %s.%d: %s", state, i, err)
				}
				rule.Regexp.MatchTimeout = time.Millisecond * 250
			}
		}
	}
restart:
	seen := map[LexerMutator]bool{}
	for state := range r.rules {
		for i := range len(r.rules[state]) {
			rule := r.rules[state][i]
			if compile, ok := rule.Mutator.(LexerMutator); ok {
				if seen[compile] {
					return fmt.Errorf("saw mutator %T twice; this should not happen", compile)
				}
				seen[compile] = true
				if err := compile.MutateLexer(r.rules, state, i); err != nil {
					return err
				}
				// Process the rules again in case the mutator added/removed rules.
				//
				// This sounds bad, but shouldn't be significant in practice.
				goto restart
			}
		}
	}
	// Validate emitters
	for state := range r.rules {
		for i := range len(r.rules[state]) {
			rule := r.rules[state][i]
			if validate, ok := rule.Type.(ValidatingEmitter); ok {
				if err := validate.ValidateEmitter(rule); err != nil {
					return fmt.Errorf("%s: %s: %s: %w", r.config.Name, state, rule.Pattern, err)
				}
			}
		}
	}
	r.compiled = true
	return nil
}

func (r *RegexLexer) fetchRules() error {
	rules, err := r.fetchRulesFunc()
	if err != nil {
		return fmt.Errorf("%s: failed to compile rules: %w", r.config.Name, err)
	}
	if _, ok := rules["root"]; !ok {
		return fmt.Errorf("no \"root\" state")
	}
	compiledRules := map[string][]*CompiledRule{}
	for state, rules := range rules {
		compiledRules[state] = nil
		for _, rule := range rules {
			flags := ""
			if !r.config.NotMultiline {
				flags += "m"
			}
			if r.config.CaseInsensitive {
				flags += "i"
			}
			if r.config.DotAll {
				flags += "s"
			}
			compiledRules[state] = append(compiledRules[state], &CompiledRule{Rule: rule, flags: flags})
		}
	}

	r.rawRules = rules
	r.rules = compiledRules
	return nil
}

func (r *RegexLexer) needRules() error {
	var err error
	if r.fetchRulesFunc != nil {
		r.compileOnce.Do(func() {
			r.compileError = r.fetchRules()
		})
		if r.compileError != nil {
			return r.compileError
		}
	}
	if err := r.maybeCompile(); err != nil {
		return err
	}
	return err
}

// Tokenise text using lexer, returning an iterator.
func (r *RegexLexer) Tokenise(options *TokeniseOptions, text string) (Iterator, error) {
	err := r.needRules()
	if err != nil {
		return nil, err
	}
	if options == nil {
		options = defaultOptions
	}
	if options.EnsureLF {
		text = ensureLF(text)
	}
	newlineAdded := false
	if !options.Nested && r.config.EnsureNL && !strings.HasSuffix(text, "\n") {
		text += "\n"
		newlineAdded = true
	}
	state := &LexerState{
		Registry:       r.registry,
		newlineAdded:   newlineAdded,
		options:        options,
		Lexer:          r,
		Text:           []rune(text),
		Stack:          []string{options.State},
		Rules:          r.rules,
		MutatorContext: map[interface{}]interface{}{},
	}
	return state.Iterator, nil
}

// MustRules is like Rules() but will panic on error.
func (r *RegexLexer) MustRules() Rules {
	rules, err := r.Rules()
	if err != nil {
		panic(err)
	}
	return rules
}

func matchRules(text []rune, pos int, rules []*CompiledRule) (int, *CompiledRule, []string, map[string]string) {
	for i, rule := range rules {
		match, err := rule.Regexp.FindRunesMatchStartingAt(text, pos)
		if match != nil && err == nil && match.Index == pos {
			groups := []string{}
			namedGroups := make(map[string]string)
			for _, g := range match.Groups() {
				namedGroups[g.Name] = g.String()
				groups = append(groups, g.String())
			}
			return i, rule, groups, namedGroups
		}
	}
	return 0, &CompiledRule{}, nil, nil
}

// replace \r and \r\n with \n
// same as strings.ReplaceAll but more efficient
func ensureLF(text string) string {
	buf := make([]byte, len(text))
	var j int
	for i := range len(text) {
		c := text[i]
		if c == '\r' {
			if i < len(text)-1 && text[i+1] == '\n' {
				continue
			}
			c = '\n'
		}
		buf[j] = c
		j++
	}
	return string(buf[:j])
}
