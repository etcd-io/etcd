package pattern

import (
	"regexp"
	"strings"

	"dev.gaijin.team/go/golib/e"
	"dev.gaijin.team/go/golib/fields"
)

// List represents a collection of compiled regular expressions that can be used
// for pattern matching against strings. It implements the flag.Value interface
// to support command-line flag binding.
type List []*regexp.Regexp //nolint:recvcheck

// NewList creates a new List from the provided regular expression patterns.
// Each pattern string is compiled into a regular expression. If any pattern
// is empty or fails to compile, an error is returned.
func NewList(patterns ...string) (List, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	list := make(List, 0, len(patterns))

	for _, pattern := range patterns {
		re, err := parseRx(pattern)
		if err != nil {
			return nil, err
		}

		list = append(list, re)
	}

	return list, nil
}

// MatchFullString checks if any of the regular expressions in the list matches
// the entire input string. A match is considered successful only if the regex
// matches the complete string from start to end, not just a substring.
//
// For example, if a List contains the pattern "test", it will match "test"
// but not "testing" or "contest".
func (l List) MatchFullString(str string) bool {
	for i := 0; i < len(l); i++ {
		if m := l[i].FindStringSubmatch(str); len(m) > 0 && m[0] == str {
			return true
		}
	}

	return false
}

// String returns a string representation of the List by joining all regex patterns
// with commas. This method implements the flag.Value interface and is used when
// the List is displayed or serialized.
func (l List) String() string {
	patterns := make([]string, len(l))
	for i, re := range l {
		patterns[i] = re.String()
	}

	return strings.Join(patterns, ",")
}

// Set adds a new regex pattern to the List by compiling the provided string.
// This method implements the flag.Value interface and is called when the flag
// is set from command-line arguments or programmatically.
func (l *List) Set(value string) error {
	re, err := parseRx(value)
	if err != nil {
		return err
	}

	*l = append(*l, re)

	return nil
}

func parseRx(str string) (*regexp.Regexp, error) {
	if str == "" {
		return nil, e.New("empty regular expression is not allowed")
	}

	re, err := regexp.Compile(str)
	if err != nil {
		return nil, e.NewFrom("failed to compile regular expression", err, fields.F("pattern", str))
	}

	return re, nil
}
