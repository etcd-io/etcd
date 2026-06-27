// Copyright 2021 The Go Authors. All rights reserved.

// This file is a modified copy of Go's encoding/json/v2/field.go

package sa5008

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
)

func validateJSONTag(pass *analysis.Pass, field *ast.Field, tag string) {
	hasTag := tag != ""
	tagOrig := tag

	// Check whether this field is explicitly ignored.
	if tag == "-" {
		return
	}

	// Check whether this field is unexported and not embedded,
	// which Go reflection cannot mutate for the sake of serialization.
	//
	// An embedded field of an unexported type is still capable of
	// forwarding exported fields, which may be JSON serialized.
	// This technically operates on the edge of what is permissible by
	// the Go language, but the most recent decision is to permit this.
	//
	// See https://go.dev/issue/24153 and https://go.dev/issue/32772.
	anonymous := len(field.Names) == 0
	if !anonymous && !field.Names[0].IsExported() {
		// Tag options specified on an unexported field suggests user error.
		if hasTag {
			report.Report(pass, field.Tag,
				fmt.Sprintf("unexported struct field cannot have non-ignored `json:%q` tag", tag))
		}
		return
	}

	if len(tag) > 0 && !strings.HasPrefix(tag, ",") {
		// For better compatibility with v1, accept almost any unescaped name.
		n := len(tag) - len(strings.TrimLeftFunc(tag, func(r rune) bool {
			return !strings.ContainsRune(",\\'\"`", r) // reserve comma, backslash, and quotes
		}))
		name := tag[:n]

		// If the next character is not a comma, then the name is either
		// malformed (if n > 0) or a single-quoted name.
		// In either case, call consumeTagOption to handle it further.
		var err error
		if !strings.HasPrefix(tag[n:], ",") && len(name) != len(tag) {
			name, n, err = consumeTagOption(tag)
			if err != nil {
				report.Report(pass, field.Tag, fmt.Sprintf("malformed `json` tag: %v", err))
			}
		}
		if !utf8.ValidString(name) {
			report.Report(pass, field.Tag,
				fmt.Sprintf("invalid UTF-8 in JSON object name %q", name))
			name = string([]rune(name)) // replace invalid UTF-8 with utf8.RuneError
		}
		if name == "-" && tag[0] == '-' {
			// TODO(dh): offer automatic fix
			report.Report(pass, field.Tag,
				fmt.Sprintf("should encoding/json ignore this field or name it \"-\"? Either use `json:\"-\"` to ignore the field or use `json:\"'-'%s` to specify %q as the name",
					strings.TrimPrefix(strconv.Quote(tagOrig), `"-`), name))
		}
		tag = tag[n:]
	}

	// Handle any additional tag options (if any).
	var wasFormat bool
	seenOpts := make(map[string]bool)
	for len(tag) > 0 {
		// Consume comma delimiter.
		if tag[0] != ',' {
			report.Report(pass, field.Tag,
				fmt.Sprintf("malformed `json` tag: invalid character %q before next option (expecting ',')",
					tag[0]))
		} else {
			tag = tag[len(","):]
			if len(tag) == 0 {
				report.Report(pass, field.Tag, "malformed `json` tag: invalid trailing ',' character")
				break
			}
		}

		// Consume and process the tag option.
		opt, n, err := consumeTagOption(tag)
		if err != nil {
			report.Report(pass, field.Tag, fmt.Sprintf("malformed `json` tag: %v", err))
		}
		rawOpt := tag[:n]
		tag = tag[n:]
		switch {
		case wasFormat:
			report.Report(pass, field.Tag, "`format` tag option was not specified last")
		case strings.HasPrefix(rawOpt, "'") && strings.TrimFunc(opt, isLetterOrDigit) == "":
			// TODO(dh): offer automatic fix
			report.Report(pass, field.Tag,
				fmt.Sprintf("unnecessarily quoted appearance of `%s` tag option; specify `%s` instead", rawOpt, opt))
		}
		switch opt {
		case "case":
			if !strings.HasPrefix(tag, ":") {
				// TODO(dh): offer automatic fix
				report.Report(pass, field.Tag,
					"missing value for `case` tag option; specify `case:ignore` or `case:strict` instead")
				break
			}
			tag = tag[len(":"):]
			opt, n, err := consumeTagOption(tag)
			if err != nil {
				report.Report(pass, field.Tag,
					fmt.Sprintf("malformed value for `case` tag option: %v", err))
				break
			}
			rawOpt := tag[:n]
			tag = tag[n:]
			if strings.HasPrefix(rawOpt, "'") {
				// TODO(dh): offer automatic fix
				report.Report(pass, field.Tag,
					fmt.Sprintf("unnecessarily quoted appearance of `case:%s` tag option; specify `case:%s` instead",
						rawOpt, opt))
			}
			switch opt {
			case "ignore":
			case "strict":
			default:
				report.Report(pass, field.Tag,
					fmt.Sprintf("invalid appearance of unknown `case:%s` tag value", rawOpt))
			}
		case "inline":
		case "unknown":
		case "omitzero":
		case "omitempty":
		case "string":
			const msg = "invalid appearance of `string` tag option; it is only intended for fields of numeric types or pointers to those"
			tset := typeutil.NewTypeSet(pass.TypesInfo.TypeOf(field.Type))
			if len(tset.Terms) == 0 {
				// TODO(dh): improve message, call out the use of type parameters
				report.Report(pass, field.Tag, msg)
				continue
			}
			for _, term := range tset.Terms {
				T := typeutil.Dereference(term.Type().Underlying())
				for _, term2 := range typeutil.NewTypeSet(T).Terms {
					basic, ok := term2.Type().Underlying().(*types.Basic)
					// We accept bools and strings because v1 of encoding/json
					// supports those. We don't mention that in the message,
					// however, because their support is accidental, and v2
					// doesn't support it.
					if !ok || (basic.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsString)) == 0 {
						// TODO(dh): improve message, show how we arrived at the type
						report.Report(pass, field.Tag, msg)
					}
				}
			}
		case "format":
			if !strings.HasPrefix(tag, ":") {
				report.Report(pass, field.Tag, "missing value for `format` tag option")
				break
			}
			tag = tag[len(":"):]
			_, n, err := consumeTagOption(tag)
			if err != nil {
				report.Report(pass, field.Tag,
					fmt.Sprintf("malformed value for `format` tag option: %v", err))
				break
			}
			tag = tag[n:]
			wasFormat = true
		default:
			// Reject keys that resemble one of the supported options.
			// This catches invalid mutants such as "omitEmpty" or "omit_empty".
			normOpt := strings.ReplaceAll(strings.ToLower(opt), "_", "")
			switch normOpt {
			case "case", "inline", "unknown", "omitzero", "omitempty", "string", "format":
				report.Report(pass, field.Tag,
					fmt.Sprintf("invalid appearance of `%s` tag option; specify `%s` instead",
						opt, normOpt))
			default:
				report.Report(pass, field.Tag,
					fmt.Sprintf("invalid appearance of unknown `%s` tag option", opt))
			}
		}

		// Reject duplicates.
		if seenOpts[opt] {
			report.Report(pass, field.Tag,
				fmt.Sprintf("duplicate appearance of `%s` tag option", rawOpt))
		}
		seenOpts[opt] = true
	}

	if seenOpts["inline"] && seenOpts["unknown"] {
		report.Report(pass, field.Tag,
			"field cannot have both `inline` and `unknown` specified")
	}

	// TODO(dh): implement more restrictions for types of inlined and unknown
	// fields, including recursive restrictions:
	//
	// - Go struct field %s cannot have any options other than `inline` or `unknown` specified
	// - inlined Go struct field %s of type %s with `unknown` tag must be a Go map of string key or a jsontext.Value
	// - inlined Go struct field %s is not exported
	// - inlined map field %s of type %s must have a string key that does not implement marshal or unmarshal methods
	// - inlined Go struct field %s of type %s must be a Go struct, Go map of string key, or jsontext.Value
}

// consumeTagOption consumes the next option,
// which is either a Go identifier or a single-quoted string.
// If the next option is invalid, it returns all of in until the next comma,
// and reports an error.
func consumeTagOption(in string) (string, int, error) {
	// For legacy compatibility with v1, assume options are comma-separated.
	i := strings.IndexByte(in, ',')
	if i < 0 {
		i = len(in)
	}

	switch r, _ := utf8.DecodeRuneInString(in); {
	// Option as a Go identifier.
	case r == '_' || unicode.IsLetter(r):
		n := len(in) - len(strings.TrimLeftFunc(in, isLetterOrDigit))
		return in[:n], n, nil
	// Option as a single-quoted string.
	case r == '\'':
		// The grammar is nearly identical to a double-quoted Go string literal,
		// but uses single quotes as the terminators. The reason for a custom
		// grammar is because both backtick and double quotes cannot be used
		// verbatim in a struct tag.
		//
		// Convert a single-quoted string to a double-quote string and rely on
		// strconv.Unquote to handle the rest.
		var inEscape bool
		b := []byte{'"'}
		n := len(`'`)
		for len(in) > n {
			r, rn := utf8.DecodeRuneInString(in[n:])
			switch {
			case inEscape:
				if r == '\'' {
					b = b[:len(b)-1] // remove escape character: `\'` => `'`
				}
				inEscape = false
			case r == '\\':
				inEscape = true
			case r == '"':
				b = append(b, '\\') // insert escape character: `"` => `\"`
			case r == '\'':
				b = append(b, '"')
				n += len(`'`)
				out, err := strconv.Unquote(string(b))
				if err != nil {
					return in[:i], i, fmt.Errorf("invalid single-quoted string: %s", in[:n])
				}
				return out, n, nil
			}
			b = append(b, in[n:][:rn]...)
			n += rn
		}
		if n > 10 {
			n = 10 // limit the amount of context printed in the error
		}
		//lint:ignore ST1005 The ellipsis denotes truncated text
		return in[:i], i, fmt.Errorf("single-quoted string not terminated: %s...", in[:n])
	case len(in) == 0:
		return in[:i], i, io.ErrUnexpectedEOF
	default:
		return in[:i], i, fmt.Errorf("invalid character %q at start of option (expecting Unicode letter or single quote)", r)
	}
}

func isLetterOrDigit(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}
