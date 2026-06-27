package recvcheck

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// NewAnalyzer returns a new analyzer to check for receiver type consistency.
func NewAnalyzer(s Settings) *analysis.Analyzer {
	a := &analyzer{
		excluded: map[string]struct{}{},
	}

	if !s.DisableBuiltin {
		// Default excludes for Marshal/Encode methods https://github.com/raeperd/recvcheck/issues/7
		a.excluded = map[string]struct{}{
			"*.MarshalText":   {},
			"*.MarshalJSON":   {},
			"*.MarshalYAML":   {},
			"*.MarshalXML":    {},
			"*.MarshalBinary": {},
			"*.GobEncode":     {},
		}
	}

	for _, exclusion := range s.Exclusions {
		a.excluded[exclusion] = struct{}{}
	}

	return &analysis.Analyzer{
		Name:     "recvcheck",
		Doc:      "checks for receiver type consistency",
		Run:      a.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

// Settings is the configuration for the analyzer.
type Settings struct {
	// DisableBuiltin if true, disables the built-in method excludes.
	// Built-in excluded methods:
	//   - "MarshalText"
	//   - "MarshalJSON"
	//   - "MarshalYAML"
	//   - "MarshalXML"
	//   - "MarshalBinary"
	//   - "GobEncode"
	DisableBuiltin bool

	// Exclusions format is `struct_name.method_name` (ex: `Foo.MethodName`).
	// A wildcard `*` can use as a struct name (ex: `*.MethodName`).
	Exclusions []string
}

type analyzer struct {
	excluded map[string]struct{}
}

func (r *analyzer) run(pass *analysis.Pass) (any, error) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	structs := map[string]*structType{}
	inspector.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) != 1 {
			return
		}

		recv, isStar := recvTypeIdent(funcDecl.Recv.List[0].Type)
		if recv == nil {
			return
		}

		if r.isExcluded(recv, funcDecl) {
			return
		}

		st, ok := structs[recv.Name]
		if !ok {
			structs[recv.Name] = &structType{}
			st = structs[recv.Name]
		}

		if isStar {
			st.starUsed = true
		} else {
			st.typeUsed = true
		}
	})

	for recv, st := range structs {
		if st.starUsed && st.typeUsed {
			pass.Reportf(pass.Pkg.Scope().Lookup(recv).Pos(), "the methods of %q use pointer receiver and non-pointer receiver.", recv)
		}
	}

	return nil, nil
}

func (r *analyzer) isExcluded(recv *ast.Ident, f *ast.FuncDecl) bool {
	if f.Name == nil || f.Name.Name == "" {
		return true
	}

	_, found := r.excluded[recv.Name+"."+f.Name.Name]
	if found {
		return true
	}

	_, found = r.excluded["*."+f.Name.Name]

	return found
}

type structType struct {
	starUsed bool
	typeUsed bool
}

func recvTypeIdent(r ast.Expr) (*ast.Ident, bool) {
	switch n := r.(type) {
	case *ast.StarExpr:
		if i, ok := n.X.(*ast.Ident); ok {
			return i, true
		}

	case *ast.Ident:
		return n, false
	}

	return nil, false
}
