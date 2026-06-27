package checkers

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// SuiteTHelper requires t.Helper() call in suite helpers:
//
//	func (s *RoomSuite) assertRoomRound(roundID RoundID) {
//		s.T().Helper()
//		s.Equal(roundID, s.getRoom().CurrentRound.ID)
//	}
type SuiteTHelper struct{}

// NewSuiteTHelper constructs SuiteTHelper checker.
func NewSuiteTHelper() SuiteTHelper { return SuiteTHelper{} }
func (SuiteTHelper) Name() string   { return "suite-thelper" }

func (checker SuiteTHelper) Check(pass *analysis.Pass, insp *inspector.Inspector) (diagnostics []analysis.Diagnostic) {
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(node ast.Node) {
		fd := node.(*ast.FuncDecl)
		if !isSuiteMethod(pass, fd) {
			return
		}

		if ident := fd.Name; ident == nil || isSuiteTestMethod(ident.Name) || isSuiteServiceMethod(ident.Name) {
			return
		}

		if !fnContainsAssertions(pass, fd) {
			return
		}

		rcv := fd.Recv.List[0]
		if len(rcv.Names) != 1 || rcv.Names[0] == nil {
			return
		}
		rcvName := rcv.Names[0].Name

		helperCallStr := rcvName + ".T().Helper()"

		firstStmt := fd.Body.List[0]
		if analysisutil.NodeString(pass.Fset, firstStmt) == helperCallStr {
			return
		}

		msg := "suite helper method must start with " + helperCallStr
		d := newDiagnostic(checker.Name(), fd, msg, analysis.SuggestedFix{
			Message: fmt.Sprintf("Insert `%s`", helperCallStr),
			TextEdits: []analysis.TextEdit{
				{
					Pos:     firstStmt.Pos(),
					End:     firstStmt.Pos(), // Pure insertion.
					NewText: []byte(helperCallStr + "\n\n"),
				},
			},
		})
		diagnostics = append(diagnostics, *d)
	})
	return diagnostics
}
