package reports

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/nunnatsa/ginkgolinter/internal/formatter"

	"golang.org/x/tools/go/analysis"
)

type Builder struct {
	pos        token.Pos
	end        token.Pos
	oldExpr    string
	issues     []string
	fixOffer   string
	suggestFix bool
	formatter  *formatter.GoFmtFormatter
}

func NewBuilder(oldExpr ast.Expr, expFormatter *formatter.GoFmtFormatter) *Builder {
	b := &Builder{
		pos:        oldExpr.Pos(),
		end:        oldExpr.End(),
		oldExpr:    expFormatter.Format(oldExpr),
		suggestFix: false,
		formatter:  expFormatter,
	}

	return b
}

func (b *Builder) OldExp() string {
	return b.oldExpr
}

func (b *Builder) AddIssue(suggestFix bool, issue string, args ...any) {
	if len(args) > 0 {
		issue = fmt.Sprintf(issue, args...)
	}
	b.issues = append(b.issues, issue)

	if suggestFix {
		b.suggestFix = true
	}
}

func (b *Builder) SetFixOffer(fixOffer ast.Expr) {
	if b.suggestFix {
		if offer := b.formatter.Format(fixOffer); offer != b.oldExpr {
			b.fixOffer = offer
		}
	}
}

func (b *Builder) HasReport() bool {
	return len(b.issues) > 0
}

func (b *Builder) Build() analysis.Diagnostic {
	diagnostic := analysis.Diagnostic{
		Pos:     b.pos,
		Message: b.getMessage(),
	}

	if b.suggestFix && len(b.fixOffer) > 0 {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{
			{
				Message: fmt.Sprintf("should replace %s with %s", b.oldExpr, b.fixOffer),
				TextEdits: []analysis.TextEdit{
					{
						Pos:     b.pos,
						End:     b.end,
						NewText: []byte(b.fixOffer),
					},
				},
			},
		}
	}

	return diagnostic
}

func (b *Builder) FormatExpr(expr ast.Expr) string {
	return b.formatter.Format(expr)
}

func (b *Builder) getMessage() string {
	sb := strings.Builder{}
	sb.WriteString("ginkgo-linter: ")
	if len(b.issues) > 1 {
		sb.WriteString("multiple issues: ")
	}
	sb.WriteString(strings.Join(b.issues, "; "))

	if b.suggestFix && len(b.fixOffer) != 0 {
		sb.WriteString(fmt.Sprintf(". Consider using `%s` instead", b.fixOffer))
	}

	return sb.String()
}
