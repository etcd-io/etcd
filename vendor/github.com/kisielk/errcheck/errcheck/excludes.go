package errcheck

import (
	"bufio"
	"bytes"
	"os"
	"strings"
)

// DefaultExcludedSymbols is a list of symbol names that are usually excluded from checks by default.
//
// Note, that they still need to be explicitly copied to Checker.Exclusions.Symbols
var DefaultExcludedSymbols = []string{
	// bytes
	"(*bytes.Buffer).Write",
	"(*bytes.Buffer).WriteByte",
	"(*bytes.Buffer).WriteRune",
	"(*bytes.Buffer).WriteString",

	// crypto
	"crypto/rand.Read", // https://github.com/golang/go/issues/66821

	// fmt
	"fmt.Print",
	"fmt.Printf",
	"fmt.Println",
	"fmt.Fprint(*bytes.Buffer)",
	"fmt.Fprintf(*bytes.Buffer)",
	"fmt.Fprintln(*bytes.Buffer)",
	"fmt.Fprint(*strings.Builder)",
	"fmt.Fprintf(*strings.Builder)",
	"fmt.Fprintln(*strings.Builder)",
	"fmt.Fprint(os.Stderr)",
	"fmt.Fprintf(os.Stderr)",
	"fmt.Fprintln(os.Stderr)",

	// io
	"(*io.PipeReader).CloseWithError",
	"(*io.PipeWriter).CloseWithError",

	// math/rand
	"math/rand.Read",
	"(*math/rand.Rand).Read",

	// strings
	"(*strings.Builder).Write",
	"(*strings.Builder).WriteByte",
	"(*strings.Builder).WriteRune",
	"(*strings.Builder).WriteString",

	// hash
	"(hash.Hash).Write",

	// hash/maphash
	"(*hash/maphash.Hash).Write",
	"(*hash/maphash.Hash).WriteByte",
	"(*hash/maphash.Hash).WriteString",
}

// ReadExcludes reads an excludes file, a newline delimited file that lists
// patterns for which to allow unchecked errors.
//
// Lines that start with two forward slashes are considered comments and are ignored.
func ReadExcludes(path string) ([]string, error) {
	var excludes []string

	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(buf))

	for scanner.Scan() {
		name := scanner.Text()
		// Skip comments and empty lines.
		if strings.HasPrefix(name, "//") || name == "" {
			continue
		}
		excludes = append(excludes, name)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return excludes, nil
}
