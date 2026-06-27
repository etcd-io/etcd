package rule

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// ModifiesValRecRule lints assignments to value method-receivers.
type ModifiesValRecRule struct{}

// Apply applies the rule to given file.
func (r *ModifiesValRecRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	file.Pkg.TypeCheck()
	for _, decl := range file.AST.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		isAMethod := ok && funcDecl.Recv != nil
		if !isAMethod {
			continue // skip, not a method
		}

		receiver := funcDecl.Recv.List[0]
		if r.mustSkip(receiver, file.Pkg) {
			continue
		}

		receiverName := receiver.Names[0].Name
		assignmentsToReceiver := r.getReceiverModifications(receiverName, funcDecl.Body)
		if len(assignmentsToReceiver) == 0 {
			continue // receiver is not modified
		}

		methodReturnsReceiver := r.seekReturnReceiverStatement(receiverName, funcDecl.Body) != nil
		if methodReturnsReceiver {
			continue // modification seems legit (see issue #1066)
		}

		for _, assignment := range assignmentsToReceiver {
			failures = append(failures, lint.Failure{
				Node:       assignment,
				Confidence: 1,
				Failure:    "suspicious assignment to a by-value method receiver",
			})
		}
	}

	return failures
}

// Name returns the rule name.
func (*ModifiesValRecRule) Name() string {
	return "modifies-value-receiver"
}

func (*ModifiesValRecRule) skipType(t ast.Expr, pkg *lint.Package) bool {
	rt := pkg.TypeOf(t)
	if rt == nil {
		return false
	}

	rt = rt.Underlying()
	rtName := rt.String()

	// skip when receiver is a map or array
	return strings.HasPrefix(rtName, "[]") || strings.HasPrefix(rtName, "map[")
}

func (*ModifiesValRecRule) getNameFromExpr(ie ast.Expr) string {
	ident, ok := ie.(*ast.Ident)
	if !ok {
		return ""
	}

	return ident.Name
}

func (r *ModifiesValRecRule) seekReturnReceiverStatement(receiverName string, target ast.Node) ast.Node {
	finder := func(n ast.Node) bool {
		// look for returns with the receiver as value
		returnStatement, ok := n.(*ast.ReturnStmt)
		if !ok {
			return false
		}

		for _, exp := range returnStatement.Results {
			switch e := exp.(type) {
			case *ast.SelectorExpr: // receiver.field = ...
				name := r.getNameFromExpr(e.X)
				if name == "" || name != receiverName {
					continue
				}
			case *ast.Ident: // receiver := ...
				if e.Name != receiverName {
					continue
				}
			case *ast.UnaryExpr:
				if e.Op != token.AND {
					continue
				}
				name := r.getNameFromExpr(e.X)
				if name == "" || name != receiverName {
					continue
				}

			default:
				continue
			}

			return true
		}

		return false
	}

	return astutils.SeekNode[ast.Node](target, finder)
}

func (r *ModifiesValRecRule) mustSkip(receiver *ast.Field, pkg *lint.Package) bool {
	if _, ok := receiver.Type.(*ast.StarExpr); ok {
		return true // skip, method with pointer receiver
	}

	if len(receiver.Names) < 1 {
		return true // skip, anonymous receiver
	}

	receiverName := receiver.Names[0].Name
	if receiverName == "_" {
		return true // skip, anonymous receiver
	}

	if r.skipType(receiver.Type, pkg) {
		return true // skip, receiver is a map or array
	}

	return false
}

func (r *ModifiesValRecRule) getReceiverModifications(receiverName string, funcBody *ast.BlockStmt) []ast.Node {
	receiverModificationFinder := func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IncDecStmt:
			se, ok := node.X.(*ast.SelectorExpr)
			if !ok {
				return false
			}

			name := r.getNameFromExpr(se.X)
			return name == receiverName
		case *ast.AssignStmt:
			// look for assignments with the receiver in the right hand
			for _, exp := range node.Lhs {
				switch e := exp.(type) {
				case *ast.IndexExpr: // receiver...[] = ...
					continue
				case *ast.StarExpr: // *receiver = ...
					continue
				case *ast.SelectorExpr: // receiver.field = ...
					name := r.getNameFromExpr(e.X)
					if name == "" || name != receiverName {
						continue
					}
				case *ast.Ident: // receiver := ...
					if e.Name != receiverName {
						continue
					}
				default:
					continue
				}

				return true
			}
		}

		return false
	}

	return astutils.PickNodes(funcBody, receiverModificationFinder)
}
