package sa4020

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/exp/typeparams"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4020",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Unreachable case clause in a type switch`,
		Text: `In a type switch like the following

    type T struct{}
    func (T) Read(b []byte) (int, error) { return 0, nil }

    var v any = T{}

    switch v.(type) {
    case io.Reader:
        // ...
    case T:
        // unreachable
    }

the second case clause can never be reached because \'T\' implements
\'io.Reader\' and case clauses are evaluated in source order.

Another example:

    type T struct{}
    func (T) Read(b []byte) (int, error) { return 0, nil }
    func (T) Close() error { return nil }

    var v any = T{}

    switch v.(type) {
    case io.Reader:
        // ...
    case io.ReadCloser:
        // unreachable
    }

Even though \'T\' has a \'Close\' method and thus implements \'io.ReadCloser\',
\'io.Reader\' will always match first. The method set of \'io.Reader\' is a
subset of \'io.ReadCloser\'. Thus it is impossible to match the second
case without matching the first case.


Structurally equivalent interfaces

A special case of the previous example are structurally identical
interfaces. Given these declarations

    type T error
    type V error

    func doSomething() error {
        err, ok := doAnotherThing()
        if ok {
            return T(err)
        }

        return U(err)
    }

the following type switch will have an unreachable case clause:

    switch doSomething().(type) {
    case T:
        // ...
    case V:
        // unreachable
    }

\'T\' will always match before V because they are structurally equivalent
and therefore \'doSomething()\''s return value implements both.`,
		Since:    "2019.2",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	// Check if T subsumes V in a type switch. T subsumes V if T is an interface and T's method set is a subset of V's method set.
	subsumes := func(T, V types.Type) bool {
		if typeparams.IsTypeParam(T) {
			return false
		}
		tIface, ok := T.Underlying().(*types.Interface)
		if !ok {
			return false
		}

		return types.Implements(V, tIface)
	}

	subsumesAny := func(Ts, Vs []types.Type) (types.Type, types.Type, bool) {
		for _, T := range Ts {
			for _, V := range Vs {
				if subsumes(T, V) {
					return T, V, true
				}
			}
		}

		return nil, nil, false
	}

	fn := func(node ast.Node) {
		tsStmt := node.(*ast.TypeSwitchStmt)

		type ccAndTypes struct {
			cc    *ast.CaseClause
			types []types.Type
		}

		// All asserted types in the order of case clauses.
		ccs := make([]ccAndTypes, 0, len(tsStmt.Body.List))
		for _, stmt := range tsStmt.Body.List {
			cc, _ := stmt.(*ast.CaseClause)

			// Exclude the 'default' case.
			if len(cc.List) == 0 {
				continue
			}

			Ts := make([]types.Type, 0, len(cc.List))
			for _, expr := range cc.List {
				// Exclude the 'nil' value from any 'case' statement (it is always reachable).
				if typ := pass.TypesInfo.TypeOf(expr); typ != types.Typ[types.UntypedNil] {
					Ts = append(Ts, typ)
				}
			}

			ccs = append(ccs, ccAndTypes{cc: cc, types: Ts})
		}

		if len(ccs) <= 1 {
			// Zero or one case clauses, nothing to check.
			return
		}

		// Check if case clauses following cc have types that are subsumed by cc.
		for i, cc := range ccs[:len(ccs)-1] {
			for _, next := range ccs[i+1:] {
				if T, V, yes := subsumesAny(cc.types, next.types); yes {
					report.Report(pass, next.cc, fmt.Sprintf("unreachable case clause: %s will always match before %s", T.String(), V.String()),
						report.ShortRange())
				}
			}
		}
	}

	code.Preorder(pass, fn, (*ast.TypeSwitchStmt)(nil))
	return nil, nil
}
