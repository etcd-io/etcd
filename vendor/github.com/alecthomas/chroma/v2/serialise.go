package chroma

import (
	"compress/gzip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
)

// Serialisation of Chroma rules to XML. The format is:
//
//	<rules>
//	  <state name="$STATE">
//	    <rule [pattern="$PATTERN"]>
//	      [<$EMITTER ...>]
//	      [<$MUTATOR ...>]
//	    </rule>
//	  </state>
//	</rules>
//
// eg. Include("String") would become:
//
//	<rule>
//	  <include state="String" />
//	</rule>
//
//	[null, null, {"kind": "include", "state": "String"}]
//
// eg. Rule{`\d+`, Text, nil} would become:
//
//	<rule pattern="\\d+">
//	  <token type="Text"/>
//	</rule>
//
// eg. Rule{`"`, String, Push("String")}
//
//	<rule pattern="\"">
//	  <token type="String" />
//	  <push state="String" />
//	</rule>
//
// eg. Rule{`(\w+)(\n)`, ByGroups(Keyword, Whitespace), nil},
//
//	<rule pattern="(\\w+)(\\n)">
//	  <bygroups token="Keyword" token="Whitespace" />
//	  <push state="String" />
//	</rule>
var (
	// ErrNotSerialisable is returned if a lexer contains Rules that cannot be serialised.
	ErrNotSerialisable = fmt.Errorf("not serialisable")
	emitterTemplates   = func() map[string]SerialisableEmitter {
		out := map[string]SerialisableEmitter{}
		for _, emitter := range []SerialisableEmitter{
			&byGroupsEmitter{},
			&usingSelfEmitter{},
			TokenType(0),
			&usingEmitter{},
			&usingByGroup{},
		} {
			out[emitter.EmitterKind()] = emitter
		}
		return out
	}()
	mutatorTemplates = func() map[string]SerialisableMutator {
		out := map[string]SerialisableMutator{}
		for _, mutator := range []SerialisableMutator{
			&includeMutator{},
			&combinedMutator{},
			&multiMutator{},
			&pushMutator{},
			&popMutator{},
		} {
			out[mutator.MutatorKind()] = mutator
		}
		return out
	}()
)

// fastUnmarshalConfig unmarshals only the Config from a serialised lexer.
func fastUnmarshalConfig(from fs.FS, path string) (*Config, error) {
	r, err := from.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	dec := xml.NewDecoder(r)
	for {
		token, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("could not find <config> element")
			}
			return nil, err
		}
		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local != "config" {
				break
			}

			var config Config
			err = dec.DecodeElement(&config, &se)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			return &config, nil
		}
	}
}

// MustNewXMLLexer constructs a new RegexLexer from an XML file or panics.
func MustNewXMLLexer(from fs.FS, path string) *RegexLexer {
	lex, err := NewXMLLexer(from, path)
	if err != nil {
		panic(err)
	}
	return lex
}

// NewXMLLexer creates a new RegexLexer from a serialised RegexLexer.
func NewXMLLexer(from fs.FS, path string) (*RegexLexer, error) {
	config, err := fastUnmarshalConfig(from, path)
	if err != nil {
		return nil, err
	}

	for _, glob := range append(config.Filenames, config.AliasFilenames...) {
		_, err := filepath.Match(glob, "")
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a valid glob: %w", config.Name, glob, err)
		}
	}

	var analyserFn func(string) float32

	if config.Analyse != nil {
		type regexAnalyse struct {
			re    *regexp2.Regexp
			score float32
		}

		regexAnalysers := make([]regexAnalyse, 0, len(config.Analyse.Regexes))

		regexFlags := regexp2.None
		if config.CaseInsensitive {
			regexFlags = regexp2.IgnoreCase
		}
		for _, ra := range config.Analyse.Regexes {
			re, err := regexp2.Compile(ra.Pattern, regexFlags)
			if err != nil {
				return nil, fmt.Errorf("%s: %q is not a valid analyser regex: %w", config.Name, ra.Pattern, err)
			}

			regexAnalysers = append(regexAnalysers, regexAnalyse{re, ra.Score})
		}

		analyserFn = func(text string) float32 {
			var score float32

			for _, ra := range regexAnalysers {
				ok, err := ra.re.MatchString(text)
				if err != nil {
					return 0
				}

				if ok && config.Analyse.First {
					return float32(math.Min(float64(ra.score), 1.0))
				}

				if ok {
					score += ra.score
				}
			}

			return float32(math.Min(float64(score), 1.0))
		}
	}

	return &RegexLexer{
		config:   config,
		analyser: analyserFn,
		fetchRulesFunc: func() (Rules, error) {
			var lexer struct {
				Config
				Rules Rules `xml:"rules"`
			}
			// Try to open .xml fallback to .xml.gz
			fr, err := from.Open(path)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					path += ".gz"
					fr, err = from.Open(path)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
			defer fr.Close()
			var r io.Reader = fr
			if strings.HasSuffix(path, ".gz") {
				r, err = gzip.NewReader(r)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", path, err)
				}
			}
			err = xml.NewDecoder(r).Decode(&lexer)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			return lexer.Rules, nil
		},
	}, nil
}

// Marshal a RegexLexer to XML.
func Marshal(l *RegexLexer) ([]byte, error) {
	type lexer struct {
		Config Config `xml:"config"`
		Rules  Rules  `xml:"rules"`
	}

	rules, err := l.Rules()
	if err != nil {
		return nil, err
	}
	root := &lexer{
		Config: *l.Config(),
		Rules:  rules,
	}
	data, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`></[a-zA-Z]+>`)
	data = re.ReplaceAll(data, []byte(`/>`))
	return data, nil
}

// Unmarshal a RegexLexer from XML.
func Unmarshal(data []byte) (*RegexLexer, error) {
	type lexer struct {
		Config Config `xml:"config"`
		Rules  Rules  `xml:"rules"`
	}
	root := &lexer{}
	err := xml.Unmarshal(data, root)
	if err != nil {
		return nil, fmt.Errorf("invalid Lexer XML: %w", err)
	}
	lex, err := NewLexer(&root.Config, func() Rules { return root.Rules })
	if err != nil {
		return nil, err
	}
	return lex, nil
}

func marshalMutator(e *xml.Encoder, mutator Mutator) error {
	if mutator == nil {
		return nil
	}
	smutator, ok := mutator.(SerialisableMutator)
	if !ok {
		return fmt.Errorf("unsupported mutator: %w", ErrNotSerialisable)
	}
	return e.EncodeElement(mutator, xml.StartElement{Name: xml.Name{Local: smutator.MutatorKind()}})
}

func unmarshalMutator(d *xml.Decoder, start xml.StartElement) (Mutator, error) {
	kind := start.Name.Local
	mutator, ok := mutatorTemplates[kind]
	if !ok {
		return nil, fmt.Errorf("unknown mutator %q: %w", kind, ErrNotSerialisable)
	}
	value, target := newFromTemplate(mutator)
	if err := d.DecodeElement(target, &start); err != nil {
		return nil, err
	}
	return value().(SerialisableMutator), nil
}

func marshalEmitter(e *xml.Encoder, emitter Emitter) error {
	if emitter == nil {
		return nil
	}
	semitter, ok := emitter.(SerialisableEmitter)
	if !ok {
		return fmt.Errorf("unsupported emitter %T: %w", emitter, ErrNotSerialisable)
	}
	return e.EncodeElement(emitter, xml.StartElement{
		Name: xml.Name{Local: semitter.EmitterKind()},
	})
}

func unmarshalEmitter(d *xml.Decoder, start xml.StartElement) (Emitter, error) {
	kind := start.Name.Local
	mutator, ok := emitterTemplates[kind]
	if !ok {
		return nil, fmt.Errorf("unknown emitter %q: %w", kind, ErrNotSerialisable)
	}
	value, target := newFromTemplate(mutator)
	if err := d.DecodeElement(target, &start); err != nil {
		return nil, err
	}
	return value().(SerialisableEmitter), nil
}

func (r Rule) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "rule"},
	}
	if r.Pattern != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: "pattern"},
			Value: r.Pattern,
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := marshalEmitter(e, r.Type); err != nil {
		return err
	}
	if err := marshalMutator(e, r.Mutator); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (r *Rule) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "pattern" {
			r.Pattern = attr.Value
			break
		}
	}
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			mutator, err := unmarshalMutator(d, token)
			if err != nil && !errors.Is(err, ErrNotSerialisable) {
				return err
			} else if err == nil {
				if r.Mutator != nil {
					return fmt.Errorf("duplicate mutator")
				}
				r.Mutator = mutator
				continue
			}
			emitter, err := unmarshalEmitter(d, token)
			if err != nil && !errors.Is(err, ErrNotSerialisable) { // nolint: gocritic
				return err
			} else if err == nil {
				if r.Type != nil {
					return fmt.Errorf("duplicate emitter")
				}
				r.Type = emitter
				continue
			} else {
				return err
			}

		case xml.EndElement:
			return nil
		}
	}
}

type xmlRuleState struct {
	Name  string `xml:"name,attr"`
	Rules []Rule `xml:"rule"`
}

type xmlRules struct {
	States []xmlRuleState `xml:"state"`
}

func (r Rules) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	xr := xmlRules{}
	for state, rules := range r {
		xr.States = append(xr.States, xmlRuleState{
			Name:  state,
			Rules: rules,
		})
	}
	return e.EncodeElement(xr, xml.StartElement{Name: xml.Name{Local: "rules"}})
}

func (r *Rules) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	xr := xmlRules{}
	if err := d.DecodeElement(&xr, &start); err != nil {
		return err
	}
	if *r == nil {
		*r = Rules{}
	}
	for _, state := range xr.States {
		(*r)[state.Name] = state.Rules
	}
	return nil
}

type xmlTokenType struct {
	Type string `xml:"type,attr"`
}

func (t *TokenType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	el := xmlTokenType{}
	if err := d.DecodeElement(&el, &start); err != nil {
		return err
	}
	tt, err := TokenTypeString(el.Type)
	if err != nil {
		return err
	}
	*t = tt
	return nil
}

func (t TokenType) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "type"}, Value: t.String()})
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// This hijinks is a bit unfortunate but without it we can't deserialise into TokenType.
func newFromTemplate(template interface{}) (value func() interface{}, target interface{}) {
	t := reflect.TypeOf(template)
	if t.Kind() == reflect.Ptr {
		v := reflect.New(t.Elem())
		return v.Interface, v.Interface()
	}
	v := reflect.New(t)
	return func() interface{} { return v.Elem().Interface() }, v.Interface()
}

func (b *Emitters) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token := token.(type) {
		case xml.StartElement:
			emitter, err := unmarshalEmitter(d, token)
			if err != nil {
				return err
			}
			*b = append(*b, emitter)

		case xml.EndElement:
			return nil
		}
	}
}

func (b Emitters) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, m := range b {
		if err := marshalEmitter(e, m); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}
