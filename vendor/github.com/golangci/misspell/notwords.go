package misspell

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"
)

var (
	reEmail     = regexp.MustCompile(`[[:alnum:]_.%+-]+@[[:alnum:]-.]+\.[[:alpha:]]{2,6}[^[:alpha:]]`)
	reBackslash = regexp.MustCompile(`\\[[:lower:]]`)

	// reHost Host name regular expression.
	// The length of any one label is limited between 1 and 63 octets. (https://www.ietf.org/rfc/rfc2181.txt)
	// A TLD has at least 2 letters.
	reHost = regexp.MustCompile(`([[:alnum:]-]+\.)+[[:alpha:]]{2,63}`)
)

// RemovePath attempts to strip away embedded file system paths, e.g.
//
//	/foo/bar or /static/myimg.png
//
//	TODO: windows style.
func RemovePath(s string) string {
	out := bytes.Buffer{}

	var idx int
	for s != "" {
		if idx = strings.IndexByte(s, '/'); idx == -1 {
			out.WriteString(s)
			break
		}

		if idx > 0 {
			idx--
		}

		var chclass string

		switch s[idx] {
		case '/', ' ', '\n', '\t', '\r':
			chclass = " \n\r\t"
		case '[':
			chclass = "]\n"
		case '(':
			chclass = ")\n"
		default:
			out.WriteString(s[:idx+2])
			s = s[idx+2:]

			continue
		}

		endx := strings.IndexAny(s[idx+1:], chclass)
		if endx != -1 {
			out.WriteString(s[:idx+1])
			out.Write(bytes.Repeat([]byte{' '}, endx))
			s = s[idx+endx+1:]
		} else {
			out.WriteString(s)
			break
		}
	}

	return out.String()
}

// replaceWithBlanks returns a string with the same number of spaces as the input.
func replaceWithBlanks(s string) string {
	return strings.Repeat(" ", len(s))
}

// replaceHost same as replaceWithBlanks but if the string contains at least one uppercase letter returns the string.
// Domain names are case-insensitive but browsers and DNS convert uppercase to lower case. (https://www.ietf.org/rfc/rfc4343.txt)
func replaceHost(s string) string {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return s
		}
	}

	return replaceWithBlanks(s)
}

// RemoveEmail remove email-like strings, e.g. "nickg+junk@xfoobar.com", "nickg@xyz.abc123.biz".
func RemoveEmail(s string) string {
	return reEmail.ReplaceAllStringFunc(s, replaceWithBlanks)
}

// RemoveHost removes host-like strings "foobar.com" "abc123.fo1231.biz".
func RemoveHost(s string) string {
	return reHost.ReplaceAllStringFunc(s, replaceHost)
}

// RemoveBackslashEscapes removes characters that are preceded by a backslash.
// commonly found in printf format string "\nto".
func removeBackslashEscapes(s string) string {
	return reBackslash.ReplaceAllStringFunc(s, replaceWithBlanks)
}

// RemoveNotWords blanks out all the not words.
func RemoveNotWords(s string) string {
	// do most selective/specific first
	return removeBackslashEscapes(RemoveHost(RemoveEmail(RemovePath(StripURL(s)))))
}
