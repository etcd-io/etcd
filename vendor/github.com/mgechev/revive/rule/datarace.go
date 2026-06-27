package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

//nolint:staticcheck // TODO: ast.Object is deprecated
type nodeUID *ast.Object // type of the unique id for AST nodes

// DataRaceRule spots potential dataraces caused by goroutines capturing (by-reference)
// particular identifiers of the function from which goroutines are created.
type DataRaceRule struct{}

// Apply applies the rule to given file.
func (r *DataRaceRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	isGo122 := file.Pkg.IsAtLeastGoVersion(lint.Go122)
	var failures []lint.Failure
	for _, decl := range file.AST.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			continue // not function declaration or empty function
		}

		funcResults := funcDecl.Type.Results

		returnIDs := map[nodeUID]struct{}{}
		if funcResults != nil {
			returnIDs = r.extractReturnIDs(funcResults.List)
		}

		onFailure := func(failure lint.Failure) {
			failures = append(failures, failure)
		}

		fl := &lintFunctionForDataRaces{
			onFailure: onFailure,
			returnIDs: returnIDs,
			rangeIDs:  map[nodeUID]struct{}{},
			go122for:  isGo122,
		}

		ast.Walk(fl, funcDecl.Body)
	}

	return failures
}

// Name returns the rule name.
func (*DataRaceRule) Name() string {
	return "datarace"
}

func (*DataRaceRule) extractReturnIDs(fields []*ast.Field) map[nodeUID]struct{} {
	r := map[nodeUID]struct{}{}
	for _, f := range fields {
		for _, id := range f.Names {
			r[id.Obj] = struct{}{}
		}
	}

	return r
}

type lintFunctionForDataRaces struct {
	onFailure func(failure lint.Failure)
	returnIDs map[nodeUID]struct{}
	rangeIDs  map[nodeUID]struct{}

	go122for bool
}

func (w lintFunctionForDataRaces) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.RangeStmt:
		if n.Body == nil {
			return nil
		}

		getIDs := func(exprs ...ast.Expr) []*ast.Ident {
			r := []*ast.Ident{}
			for _, expr := range exprs {
				if id, ok := expr.(*ast.Ident); ok {
					r = append(r, id)
				}
			}
			return r
		}

		ids := getIDs(n.Key, n.Value)
		for _, id := range ids {
			w.rangeIDs[id.Obj] = struct{}{}
		}

		ast.Walk(w, n.Body)

		for _, id := range ids {
			delete(w.rangeIDs, id.Obj)
		}

		return nil // do not visit the body of the range, it has been already visited
	case *ast.GoStmt:
		f := n.Call.Fun
		funcLit, ok := f.(*ast.FuncLit)
		if !ok {
			return nil
		}
		selectIDs := func(n ast.Node) bool {
			_, ok := n.(*ast.Ident)
			return ok
		}

		ids := astutils.PickNodes(funcLit.Body, selectIDs)
		for _, id := range ids {
			id := id.(*ast.Ident)
			_, isRangeID := w.rangeIDs[id.Obj]
			_, isReturnID := w.returnIDs[id.Obj]

			switch {
			case isRangeID && !w.go122for:
				w.onFailure(lint.Failure{
					Confidence: 1,
					Node:       id,
					Category:   lint.FailureCategoryLogic,
					Failure:    fmt.Sprintf("datarace: range value %s is captured (by-reference) in goroutine", id.Name),
				})
			case isReturnID:
				w.onFailure(lint.Failure{
					Confidence: 0.8,
					Node:       id,
					Category:   lint.FailureCategoryLogic,
					Failure:    fmt.Sprintf("potential datarace: return value %s is captured (by-reference) in goroutine", id.Name),
				})
			}
		}

		return nil
	}

	return w
}
