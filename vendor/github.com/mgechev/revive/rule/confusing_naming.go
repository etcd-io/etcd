package rule

import (
	"fmt"
	"go/ast"
	"strings"
	"sync"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

type referenceMethod struct {
	fileName string
	id       *ast.Ident
}

type pkgMethods struct {
	pkg     *lint.Package
	methods map[string]map[string]*referenceMethod
	mu      *sync.Mutex
}

type packages struct {
	pkgs []pkgMethods
	mu   sync.Mutex
}

func (ps *packages) methodNames(lp *lint.Package) pkgMethods {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, pkg := range ps.pkgs {
		if pkg.pkg == lp {
			return pkg
		}
	}

	pkgm := pkgMethods{pkg: lp, methods: map[string]map[string]*referenceMethod{}, mu: &sync.Mutex{}}
	ps.pkgs = append(ps.pkgs, pkgm)

	return pkgm
}

var allPkgs = packages{pkgs: make([]pkgMethods, 1)}

// ConfusingNamingRule lints method names that differ only by capitalization.
type ConfusingNamingRule struct{}

// Apply applies the rule to given file.
func (*ConfusingNamingRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure
	fileAst := file.AST
	pkgm := allPkgs.methodNames(file.Pkg)
	walker := lintConfusingNames{
		fileName: file.Name,
		pkgm:     pkgm,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	ast.Walk(&walker, fileAst)

	return failures
}

// Name returns the rule name.
func (*ConfusingNamingRule) Name() string {
	return "confusing-naming"
}

// checkMethodName checks if a given method/function name is similar (just case differences) to other method/function
// of the same struct/file.
func checkMethodName(holder string, id *ast.Ident, w *lintConfusingNames) {
	if id.Name == "init" && holder == defaultStructName {
		// ignore init functions
		return
	}

	pkgm := w.pkgm
	name := strings.ToUpper(id.Name)

	pkgm.mu.Lock()
	defer pkgm.mu.Unlock()

	if pkgm.methods[holder] != nil {
		if pkgm.methods[holder][name] != nil {
			refMethod := pkgm.methods[holder][name]
			// confusing names
			var kind string
			if holder == defaultStructName {
				kind = "function"
			} else {
				kind = "method"
			}
			var fileName string
			if w.fileName == refMethod.fileName {
				fileName = "the same source file"
			} else {
				fileName = refMethod.fileName
			}
			w.onFailure(lint.Failure{
				Failure:    fmt.Sprintf("Method '%s' differs only by capitalization to %s '%s' in %s", id.Name, kind, refMethod.id.Name, fileName),
				Confidence: 1,
				Node:       id,
				Category:   lint.FailureCategoryNaming,
			})

			return
		}
	} else {
		pkgm.methods[holder] = make(map[string]*referenceMethod, 1)
	}

	// update the block list
	pkgm.methods[holder][name] = &referenceMethod{fileName: w.fileName, id: id}
}

type lintConfusingNames struct {
	fileName  string
	pkgm      pkgMethods
	onFailure func(lint.Failure)
}

const defaultStructName = "_" // used to map functions

// getStructName of a function receiver. Defaults to defaultStructName.
func getStructName(r *ast.FieldList) string {
	result := defaultStructName

	if r == nil || len(r.List) < 1 {
		return result
	}

	t := r.List[0].Type

	switch v := t.(type) {
	case *ast.StarExpr:
		return extractFromStarExpr(v)
	case *ast.IndexExpr:
		return extractFromIndexExpr(v)
	case *ast.Ident:
		return v.Name
	}

	return defaultStructName
}

func extractFromStarExpr(expr *ast.StarExpr) string {
	switch v := expr.X.(type) {
	case *ast.IndexExpr:
		return extractFromIndexExpr(v)
	case *ast.Ident:
		return v.Name
	}
	return defaultStructName
}

func extractFromIndexExpr(expr *ast.IndexExpr) string {
	if v, ok := expr.X.(*ast.Ident); ok {
		return v.Name
	}
	return defaultStructName
}

func checkStructFields(fields *ast.FieldList, structName string, w *lintConfusingNames) {
	bl := make(map[string]bool, len(fields.List))
	for _, f := range fields.List {
		for _, id := range f.Names {
			// Skip blank identifiers
			if id.Name == "_" {
				continue
			}

			normName := strings.ToUpper(id.Name)
			if bl[normName] {
				w.onFailure(lint.Failure{
					Failure:    fmt.Sprintf("Field '%s' differs only by capitalization to other field in the struct type %s", id.Name, structName),
					Confidence: 1,
					Node:       id,
					Category:   lint.FailureCategoryNaming,
				})
			} else {
				bl[normName] = true
			}
		}
	}
}

func (w *lintConfusingNames) Visit(n ast.Node) ast.Visitor {
	switch v := n.(type) {
	case *ast.FuncDecl:
		// Exclude naming warnings for functions that are exported to C but
		// not exported in the Go API.
		// See https://github.com/golang/lint/issues/144.
		if ast.IsExported(v.Name.Name) || !astutils.IsCgoExported(v) {
			checkMethodName(getStructName(v.Recv), v.Name, w)
		}
	case *ast.TypeSpec:
		if s, ok := v.Type.(*ast.StructType); ok {
			checkStructFields(s.Fields, v.Name.Name, w)
		}

	default:
		// will add other checks like field names, struct names, etc.
	}

	return w
}
