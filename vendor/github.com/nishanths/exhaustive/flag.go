package exhaustive

import (
	"flag"
	"regexp"
	"strings"
)

var _ flag.Value = (*regexpFlag)(nil)
var _ flag.Value = (*stringsFlag)(nil)

// regexpFlag implements flag.Value for parsing
// regular expression flag inputs.
type regexpFlag struct{ re *regexp.Regexp }

func (f *regexpFlag) String() string {
	if f == nil || f.re == nil {
		return ""
	}
	return f.re.String()
}

func (f *regexpFlag) Set(expr string) error {
	if expr == "" {
		f.re = nil
		return nil
	}

	re, err := regexp.Compile(expr)
	if err != nil {
		return err
	}

	f.re = re
	return nil
}

// stringsFlag implements flag.Value for parsing a comma-separated string
// list. Surrounding whitespace is stripped from the input and from each
// element. If filter is non-nil it is called for each element in the input.
type stringsFlag struct {
	elements []string
	filter   func(string) error
}

func (f *stringsFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(f.elements, ",")
}

func (f *stringsFlag) filterFunc() func(string) error {
	if f.filter != nil {
		return f.filter
	}
	return func(_ string) error { return nil }
}

func (f *stringsFlag) Set(input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		f.elements = nil
		return nil
	}

	for _, el := range strings.Split(input, ",") {
		el = strings.TrimSpace(el)
		if err := f.filterFunc()(el); err != nil {
			return err
		}
		f.elements = append(f.elements, el)
	}
	return nil
}
