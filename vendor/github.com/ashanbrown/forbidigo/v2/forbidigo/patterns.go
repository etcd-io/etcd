package forbidigo

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"regexp/syntax"
	"strings"

	"go.yaml.in/yaml/v3"
)

// pattern matches code that is not supposed to be used.
type pattern struct {
	re, pkgRe *regexp.Regexp

	// Pattern is the regular expression string that is used for matching.
	// It gets matched against the literal source code text or the expanded
	// text, depending on the mode in which the analyzer runs.
	Pattern string `yaml:"p"`

	// Package is a regular expression for the full package path of
	// an imported item. Ignored unless the analyzer is configured to
	// determine that information.
	Package string `yaml:"pkg,omitempty"`

	// Msg gets printed in addition to the normal message if a match is
	// found.
	Msg string `yaml:"msg,omitempty"`
}

// A yamlPattern pattern in a YAML string may be represented either by a string
// (the traditional regular expression syntax) or a struct (for more complex
// patterns).
type yamlPattern pattern

func (p *yamlPattern) UnmarshalYAML(value *yaml.Node) error {
	// Try struct first. It's unlikely that a regular expression string
	// is valid YAML for a struct.
	var ptrn pattern
	if err := unmarshalStrict(&ptrn, value); err != nil && err != io.EOF {
		errStr := err.Error()
		// Didn't work, try plain string.
		var ptrn string
		if err := unmarshalStrict(&ptrn, value); err != nil && err != io.EOF {
			return fmt.Errorf("pattern is neither a regular expression string (%s) nor a Pattern struct (%s)", err.Error(), errStr)
		}
		p.Pattern = ptrn
	} else {
		*p = yamlPattern(ptrn)
	}
	return ((*pattern)(p)).validate()
}

// unmarshalStrict implements missing yaml.UnmarshalStrict in gopkg.in/yaml.v3.
// See https://github.com/go-yaml/yaml/issues/639.
// Inspired by https://github.com/ffenix113/zigbee_home/pull/68
func unmarshalStrict(to any, node *yaml.Node) error {
	buf := &bytes.Buffer{}
	if err := yaml.NewEncoder(buf).Encode(node); err != nil {
		return err
	}

	decoder := yaml.NewDecoder(buf)
	decoder.KnownFields(true)
	return decoder.Decode(to)
}

var _ yaml.Unmarshaler = &yamlPattern{}

// parse accepts a regular expression or, if the string starts with { or contains a line break, a
// JSON or YAML representation of a Pattern.
func parse(ptrn string) (*pattern, error) {
	pattern := &pattern{}

	if strings.HasPrefix(strings.TrimSpace(ptrn), "{") ||
		strings.Contains(ptrn, "\n") {
		// Embedded JSON or YAML. We can decode both with the YAML decoder.
		decoder := yaml.NewDecoder(strings.NewReader(ptrn))
		decoder.KnownFields(true)
		if err := decoder.Decode(pattern); err != nil && err != io.EOF {
			return nil, fmt.Errorf("parsing as JSON or YAML failed: %v", err)
		}
	} else {
		pattern.Pattern = ptrn
	}

	if err := pattern.validate(); err != nil {
		return nil, err
	}
	return pattern, nil
}

func (p *pattern) validate() error {
	ptrnRe, err := regexp.Compile(p.Pattern)
	if err != nil {
		return fmt.Errorf("unable to compile source code pattern `%s`: %s", p.Pattern, err)
	}
	re, err := syntax.Parse(p.Pattern, syntax.Perl)
	if err != nil {
		return fmt.Errorf("unable to parse source code pattern `%s`: %s", p.Pattern, err)
	}
	msg := extractComment(re)
	if msg != "" {
		p.Msg = msg
	}
	p.re = ptrnRe

	if p.Package != "" {
		pkgRe, err := regexp.Compile(p.Package)
		if err != nil {
			return fmt.Errorf("unable to compile package pattern `%s`: %s", p.Package, err)
		}
		p.pkgRe = pkgRe
	}

	return nil
}

func (p *pattern) matches(matchTexts []string) bool {
	for _, text := range matchTexts {
		if p.re.MatchString(text) {
			return true
		}
	}
	return false
}

// Traverse the leaf submatches in the regex tree and extract a comment, if any
// is present.
func extractComment(re *syntax.Regexp) string {
	for _, sub := range re.Sub {
		subStr := sub.String()
		if strings.HasPrefix(subStr, "#") {
			return strings.TrimSpace(strings.TrimPrefix(sub.String(), "#"))
		}
		if len(sub.Sub) > 0 {
			if comment := extractComment(sub); comment != "" {
				return comment
			}
		}
	}
	return ""
}
