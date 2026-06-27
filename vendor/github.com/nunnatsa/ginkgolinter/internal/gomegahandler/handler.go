package gomegahandler

import (
	"go/ast"
	gotypes "go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	importPath = `"github.com/onsi/gomega"`
)

// GomegaBasicInfo is the result of [Handler.GetGomegaBasicInfo].
type GomegaBasicInfo struct {
	// MethodName is the top-level Gomega method in which an expression is rooted (e.g. Expect).
	// Empty is unknown.
	MethodName string
	// True if the expression includes an Error call.
	HasErrorMethod bool

	// RootCall is the call which is root of the expression that GetGomegaBasicInfo
	// was called for.
	//
	// This is either the function which is passed the actual value (see IsAssertion
	// and IsAsyncAssertion) or a function which is passed some expected value (see
	// IsMatcher).
	RootCall *ast.CallExpr

	// Type determines what kind of Gomega function is called.
	RootCallType CallType
}

// CallType determines what kind of function call is described by [GomegaBasicInfo].
type CallType int

const (
	// OtherCall is the unspecified type. [GetGomegaBasicInfo] never returns a GomegaBasicInfo
	// with this type.
	OtherCall CallType = iota
	// SyncAssertionCall returns gtypes.Assertion.
	SyncAssertionCall
	// AsyncAssertionCall returns gtypes.AsyncAssertion.
	AsyncAssertionCall
	// MatcherCall returns gtypes.GomegaMatcher.
	MatcherCall
)

// GetGomegaHandler returns a gomegar handler according to the way gomega was imported in the specific file
func GetGomegaHandler(file *ast.File, pass *analysis.Pass) *Handler {
	// Look up the Gomega types package. It might be imported directly or indirectly.
	gtypesPkg := getPackage(pass.Pkg, "github.com/onsi/gomega/types")
	if gtypesPkg == nil {
		return nil // No gomega import: this file does not use gomega, neither directly nor indirectly.
	}

	// Look up the interfaces. They may be nil if not in use.
	assertionInterface := lookupInterface(gtypesPkg, "Assertion")
	asyncAssertionInterface := lookupInterface(gtypesPkg, "AsyncAssertion")
	matcherInterface := lookupInterface(gtypesPkg, "GomegaMatcher")

	// If there is a direct import, then this gets replaced.
	//
	// Otherwise gomega might be used indirectly, in which case
	// we have to make an educated guess what the package name
	// should be when suggesting fixes.
	//
	// Let's assume that users would want named importing,
	// because that is the recommended approach in Go.
	//
	// In practice it doesn't really matter because the only failure
	// can be a generic "missing assertion", which doesn't
	// reference a gomega method.
	name := "gomega"
	for _, imp := range file.Imports {
		if imp.Path.Value != importPath {
			continue
		}

		switch n := imp.Name.String(); n {
		case ".":
			name = ""
		case "<nil>": // import with no local name, default is good
		default:
			name = n
		}
	}

	return &Handler{
		name:                    name,
		pass:                    pass,
		syncAssertionInterface:  assertionInterface,
		asyncAssertionInterface: asyncAssertionInterface,
		matcherInterface:        matcherInterface,
	}
}

// getPackage searches recursively for a specific package, identified by it's full import name.
//
// Surprisingly, the imports may contain cycles (unsafe.unsafe -> ... internal/runtime/sys.sys ... -> unsafe.unsafe),
// so we have to detect those. We also don't want to check the same package more than once.
func getPackage(pkg *gotypes.Package, importName string) *gotypes.Package {
	return getPackageRecursively(pkg, importName, make(map[*gotypes.Package]bool))
}

func getPackageRecursively(pkg *gotypes.Package, importName string, seen map[*gotypes.Package]bool) *gotypes.Package {
	if seen[pkg] {
		// Prevent recursion, don't check again.
		return nil
	}
	seen[pkg] = true

	// In unit testing, the actual path is a/vendor/github.com/onsi/gomega/types, so we have to relax
	// the check a bit.
	if strings.HasSuffix(pkg.Path(), importName) {
		return pkg
	}
	for _, pkg := range pkg.Imports() {
		if pkg := getPackageRecursively(pkg, importName, seen); pkg != nil {
			return pkg
		}
	}
	return nil
}

func lookupInterface(pkg *gotypes.Package, name string) *gotypes.Interface {
	def := pkg.Scope().Lookup(name)
	if def == nil {
		return nil
	}
	if i, ok := def.Type().Underlying().(*gotypes.Interface); ok {
		return i
	}
	return nil
}

// Handler provide different handling, depend on the way gomega was imported, whether
// in imported with "." name, custom name or without any name.
type Handler struct {
	// name is the name under which the gomega package was imported, empty if a dot import
	name string
	pass *analysis.Pass

	syncAssertionInterface, asyncAssertionInterface, matcherInterface *gotypes.Interface
}

// GetGomegaBasicInfo returns the name of the gomega function, e.g. `Expect` + some additional info.
//
// We identify gomega functions as:
// - The result implements a gomega interface (Assertion/AsyncAssertion/GomegaMatcher).
// - It's not a method called on such an interface.
//
// The type of the result doesn't matter, it could be interface type itself,
// type alias, struct embedding gtypes.Assertion, some other type entirely,
// etc.): if it walks like a duck, quacks like a duck, it's a duck...
//
// The method name is picked up from the identifier (dot import) or from the selector (gomega.Expect, gomega.NewWithT(t).Expect).
// Returns nil if the root call cannot be determined or does not produce one of the Gomega interfaces.
func (g *Handler) GetGomegaBasicInfo(expr *ast.CallExpr) (info *GomegaBasicInfo) {
	hasErrorMethod := false
	for {
		// Dive deeper?
		if actualFunc, ok := expr.Fun.(*ast.SelectorExpr); ok && (g.implements(actualFunc.X, g.syncAssertionInterface) || g.implements(actualFunc.X, g.asyncAssertionInterface) || g.implements(actualFunc.X, g.matcherInterface)) {
			x, ok := actualFunc.X.(*ast.CallExpr)
			if !ok {
				// Could be a variable.
				// We need to have an actual function call at the root
				// for parameter checking. We don't have one, so give up.
				return nil
			}
			if actualFunc.Sel.Name == "Error" {
				hasErrorMethod = true
			}

			// Because actualFunc.X already implemented some gomega interface,
			// actualFunc.Sel must be something like "WithOffset".
			// It's not the root, so we have to keep looking.
			expr = x
			continue
		}

		if callType := g.callType(expr); callType != OtherCall {
			// Cannot dive deeper and it returns one of the unique
			// Gomega interfaces, so this must be our root.
			info := &GomegaBasicInfo{
				HasErrorMethod: hasErrorMethod,
				RootCall:       expr,
				RootCallType:   callType,
			}
			switch actualFunc := expr.Fun.(type) {
			case *ast.Ident:
				info.MethodName = actualFunc.Name
			case *ast.SelectorExpr:
				info.MethodName = actualFunc.Sel.Name
			}
			return info
		}

		// Give up.
		return nil
	}
}

// ReplaceFunction replaces the function with another one, for fix suggestions
func (g *Handler) ReplaceFunction(caller *ast.CallExpr, newExpr *ast.Ident) {
	switch f := caller.Fun.(type) {
	case *ast.Ident:
		caller.Fun = newExpr
	case *ast.SelectorExpr:
		f.Sel = newExpr
	}
}

func (g *Handler) GetNewWrapperMatcher(name string, existing *ast.CallExpr) *ast.CallExpr {
	if g.name == "" {
		return &ast.CallExpr{
			Fun:  ast.NewIdent(name),
			Args: []ast.Expr{existing},
		}
	}

	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent(g.name),
			Sel: ast.NewIdent(name),
		},
		Args: []ast.Expr{existing},
	}
}

func (g *Handler) implements(expr ast.Expr, i *gotypes.Interface) bool {
	if i == nil {
		// Interface wasn't found -> not in use -> the expression cannot implement it.
		return false
	}
	exprType, ok := g.pass.TypesInfo.Types[expr]
	return ok && gotypes.Implements(exprType.Type, i)
}

func (g *Handler) callType(expr ast.Expr) CallType {
	switch {
	case g.implements(expr, g.syncAssertionInterface):
		return SyncAssertionCall
	case g.implements(expr, g.asyncAssertionInterface):
		return AsyncAssertionCall
	case g.implements(expr, g.matcherInterface):
		return MatcherCall
	default:
		return OtherCall
	}
}

// GetActualExprClone dives into origFunc and funcClone in lockstep until it hits
// origRootCall in origFunc, then returns the corresponding CallExpr in funcClone.
func (g *Handler) GetActualExprClone(origRootCall *ast.CallExpr, origFunc, cloneFunc *ast.SelectorExpr) (cloneRootCall *ast.CallExpr) {
	cloneExpr, ok := cloneFunc.X.(*ast.CallExpr)
	if !ok {
		return nil
	}
	origExpr, ok := origFunc.X.(*ast.CallExpr)
	if !ok {
		return nil
	}

	if origExpr == origRootCall {
		// Found it!
		return cloneExpr
	}

	cloneInnerFunc, ok := cloneExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	origInnerFunc, ok := origExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	return g.GetActualExprClone(origRootCall, origInnerFunc, cloneInnerFunc)
}
