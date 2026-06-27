package st1018

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"unicode"
	"unicode/utf8"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1018",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:   `Avoid zero-width and control characters in string literals`,
		Since:   "2019.2",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(node ast.Node) {
		lit := node.(*ast.BasicLit)
		if lit.Kind != token.STRING {
			return
		}

		type invalid struct {
			r   rune
			off int
		}
		var invalids []invalid
		hasFormat := false
		hasControl := false
		prev := rune(-1)
		const zwj = '\u200d'
		for off, r := range lit.Value {
			if unicode.Is(unicode.Cf, r) {
				if r >= '\U000e0020' && r <= '\U000e007f' {
					// These are used for spelling out country codes for flag emoji
				} else if unicode.Is(unicode.Variation_Selector, r) {
					// Always allow variation selectors
				} else if r == zwj && (unicode.Is(unicode.S, prev) || unicode.Is(unicode.Variation_Selector, prev)) {
					// Allow zero-width joiner in emoji, including those that use variation selectors.

					// Technically some foreign scripts make valid use of zero-width joiners, too, but for now we'll err
					// on the side of flagging all non-emoji uses of ZWJ.
				} else {
					switch r {
					case '\u0600', '\u0601', '\u0602', '\u0603', '\u0604', '\u0605', '\u0890', '\u0891', '\u08e2':
						// Arabic characters that are not actually invisible. If anyone knows why these are in the
						// Other, Format category please let me know.
					case '\u061c', '\u202A', '\u202B', '\u202D', '\u202E', '\u2066', '\u2067', '\u2068', '\u202C', '\u2069':
						// Bidirectional formatting characters. At best they will render confusingly, at worst they're used
						// to cause confusion.
						fallthrough
					default:
						invalids = append(invalids, invalid{r, off})
						hasFormat = true
					}
				}
			} else if unicode.Is(unicode.Cc, r) && r != '\n' && r != '\t' && r != '\r' {
				invalids = append(invalids, invalid{r, off})
				hasControl = true
			}
			prev = r
		}

		switch len(invalids) {
		case 0:
			return
		case 1:
			var kind string
			if hasFormat {
				kind = "format"
			} else if hasControl {
				kind = "control"
			} else {
				panic("unreachable")
			}

			r := invalids[0]
			msg := fmt.Sprintf("string literal contains the Unicode %s character %U, consider using the %q escape sequence instead", kind, r.r, r.r)

			replacement := strconv.QuoteRune(r.r)
			replacement = replacement[1 : len(replacement)-1]
			edit := analysis.SuggestedFix{
				Message: fmt.Sprintf("replace %s character %U with %q", kind, r.r, r.r),
				TextEdits: []analysis.TextEdit{{
					Pos:     lit.Pos() + token.Pos(r.off),
					End:     lit.Pos() + token.Pos(r.off) + token.Pos(utf8.RuneLen(r.r)),
					NewText: []byte(replacement),
				}},
			}
			delete := analysis.SuggestedFix{
				Message: fmt.Sprintf("delete %s character %U", kind, r.r),
				TextEdits: []analysis.TextEdit{{
					Pos: lit.Pos() + token.Pos(r.off),
					End: lit.Pos() + token.Pos(r.off) + token.Pos(utf8.RuneLen(r.r)),
				}},
			}
			report.Report(pass, lit, msg, report.Fixes(edit, delete))
		default:
			var kind string
			if hasFormat && hasControl {
				kind = "format and control"
			} else if hasFormat {
				kind = "format"
			} else if hasControl {
				kind = "control"
			} else {
				panic("unreachable")
			}

			msg := fmt.Sprintf("string literal contains Unicode %s characters, consider using escape sequences instead", kind)
			var edits []analysis.TextEdit
			var deletions []analysis.TextEdit
			for _, r := range invalids {
				replacement := strconv.QuoteRune(r.r)
				replacement = replacement[1 : len(replacement)-1]
				edits = append(edits, analysis.TextEdit{
					Pos:     lit.Pos() + token.Pos(r.off),
					End:     lit.Pos() + token.Pos(r.off) + token.Pos(utf8.RuneLen(r.r)),
					NewText: []byte(replacement),
				})
				deletions = append(deletions, analysis.TextEdit{
					Pos: lit.Pos() + token.Pos(r.off),
					End: lit.Pos() + token.Pos(r.off) + token.Pos(utf8.RuneLen(r.r)),
				})
			}
			edit := analysis.SuggestedFix{
				Message:   fmt.Sprintf("replace all %s characters with escape sequences", kind),
				TextEdits: edits,
			}
			delete := analysis.SuggestedFix{
				Message:   fmt.Sprintf("delete all %s characters", kind),
				TextEdits: deletions,
			}
			report.Report(pass, lit, msg, report.Fixes(edit, delete))
		}
	}
	code.Preorder(pass, fn, (*ast.BasicLit)(nil))
	return nil, nil
}
