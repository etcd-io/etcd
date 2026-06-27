// Package code answers structural and type questions about Go code.
package code

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/constant"
	"go/token"
	"go/types"
	"go/version"
	"path/filepath"
	"slices"
	"strings"

	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/facts/purity"
	"honnef.co/go/tools/analysis/facts/tokenfile"
	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/knowledge"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

type Positioner interface {
	Pos() token.Pos
}

func IsOfStringConvertibleByteSlice(pass *analysis.Pass, expr ast.Expr) bool {
	typ, ok := pass.TypesInfo.TypeOf(expr).Underlying().(*types.Slice)
	if !ok {
		return false
	}
	elem := types.Unalias(typ.Elem())
	if version.Compare(LanguageVersion(pass, expr), "go1.18") >= 0 {
		// Before Go 1.18, one could not directly convert from []T (where 'type T byte')
		// to string. See also https://github.com/golang/go/issues/23536.
		elem = elem.Underlying()
	}
	return types.Identical(elem, types.Typ[types.Byte])
}

func IsOfPointerToTypeWithName(pass *analysis.Pass, expr ast.Expr, name string) bool {
	ptr, ok := types.Unalias(pass.TypesInfo.TypeOf(expr)).(*types.Pointer)
	if !ok {
		return false
	}
	return typeutil.IsTypeWithName(ptr.Elem(), name)
}

func IsOfTypeWithName(pass *analysis.Pass, expr ast.Expr, name string) bool {
	return typeutil.IsTypeWithName(pass.TypesInfo.TypeOf(expr), name)
}

func IsInTest(pass *analysis.Pass, node Positioner) bool {
	// FIXME(dh): this doesn't work for global variables with
	// initializers
	f := pass.Fset.File(node.Pos())
	return f != nil && strings.HasSuffix(f.Name(), "_test.go")
}

// IsMain reports whether the package being processed is a package
// main.
func IsMain(pass *analysis.Pass) bool {
	return pass.Pkg.Name() == "main"
}

// IsMainLike reports whether the package being processed is a
// main-like package. A main-like package is a package that is
// package main, or that is intended to be used by a tool framework
// such as cobra to implement a command.
//
// Note that this function errs on the side of false positives; it may
// return true for packages that aren't main-like. IsMainLike is
// intended for analyses that wish to suppress diagnostics for
// main-like packages to avoid false positives.
func IsMainLike(pass *analysis.Pass) bool {
	if pass.Pkg.Name() == "main" {
		return true
	}
	for _, imp := range pass.Pkg.Imports() {
		if imp.Path() == "github.com/spf13/cobra" {
			return true
		}
	}
	return false
}

func SelectorName(pass *analysis.Pass, expr *ast.SelectorExpr) string {
	info := pass.TypesInfo
	sel := info.Selections[expr]
	if sel == nil {
		if x, ok := expr.X.(*ast.Ident); ok {
			pkg, ok := info.ObjectOf(x).(*types.PkgName)
			if !ok {
				// This shouldn't happen
				return fmt.Sprintf("%s.%s", x.Name, expr.Sel.Name)
			}
			return fmt.Sprintf("%s.%s", pkg.Imported().Path(), expr.Sel.Name)
		}
		panic(fmt.Sprintf("unsupported selector: %v", expr))
	}
	if v, ok := sel.Obj().(*types.Var); ok && v.IsField() {
		return fmt.Sprintf("(%s).%s", typeutil.DereferenceR(sel.Recv()), sel.Obj().Name())
	} else {
		return fmt.Sprintf("(%s).%s", sel.Recv(), sel.Obj().Name())
	}
}

func IsNil(pass *analysis.Pass, expr ast.Expr) bool {
	return pass.TypesInfo.Types[expr].IsNil()
}

func BoolConst(pass *analysis.Pass, expr ast.Expr) bool {
	val := pass.TypesInfo.ObjectOf(expr.(*ast.Ident)).(*types.Const).Val()
	return constant.BoolVal(val)
}

func IsBoolConst(pass *analysis.Pass, expr ast.Expr) bool {
	// We explicitly don't support typed bools because more often than
	// not, custom bool types are used as binary enums and the explicit
	// comparison is desired. We err on the side of false negatives and
	// treat aliases like other custom types.

	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	obj := pass.TypesInfo.ObjectOf(ident)
	c, ok := obj.(*types.Const)
	if !ok {
		return false
	}
	basic, ok := c.Type().(*types.Basic)
	if !ok {
		return false
	}
	if basic.Kind() != types.UntypedBool && basic.Kind() != types.Bool {
		return false
	}
	return true
}

func ExprToInt(pass *analysis.Pass, expr ast.Expr) (int64, bool) {
	tv := pass.TypesInfo.Types[expr]
	if tv.Value == nil {
		return 0, false
	}
	if tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

func ExprToString(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	val := pass.TypesInfo.Types[expr].Value
	if val == nil {
		return "", false
	}
	if val.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(val), true
}

func CallName(pass *analysis.Pass, call *ast.CallExpr) string {
	// See the comment in typeutil.FuncName for why this doesn't require special handling
	// of aliases.

	fun := astutil.Unparen(call.Fun)

	// Instantiating a function cannot return another generic function, so doing this once is enough
	switch idx := fun.(type) {
	case *ast.IndexExpr:
		fun = idx.X
	case *ast.IndexListExpr:
		fun = idx.X
	}

	// (foo)[T] is not a valid instantiation, so no need to unparen again.

	switch fun := fun.(type) {
	case *ast.SelectorExpr:
		fn, ok := pass.TypesInfo.ObjectOf(fun.Sel).(*types.Func)
		if !ok {
			return ""
		}
		return typeutil.FuncName(fn)
	case *ast.Ident:
		obj := pass.TypesInfo.ObjectOf(fun)
		switch obj := obj.(type) {
		case *types.Func:
			return typeutil.FuncName(obj)
		case *types.Builtin:
			return obj.Name()
		default:
			return ""
		}
	default:
		return ""
	}
}

func IsCallTo(pass *analysis.Pass, node ast.Node, name string) bool {
	// See the comment in typeutil.FuncName for why this doesn't require special handling
	// of aliases.

	call, ok := node.(*ast.CallExpr)
	if !ok {
		return false
	}
	return CallName(pass, call) == name
}

func IsCallToAny(pass *analysis.Pass, node ast.Node, names ...string) bool {
	// See the comment in typeutil.FuncName for why this doesn't require special handling
	// of aliases.

	call, ok := node.(*ast.CallExpr)
	if !ok {
		return false
	}
	q := CallName(pass, call)
	return slices.Contains(names, q)
}

func File(pass *analysis.Pass, node Positioner) *ast.File {
	m := pass.ResultOf[tokenfile.Analyzer].(map[*token.File]*ast.File)
	return m[pass.Fset.File(node.Pos())]
}

// BuildConstraints returns the build constraints for file f. It considers both //go:build lines as well as
// GOOS and GOARCH in file names.
func BuildConstraints(pass *analysis.Pass, f *ast.File) (constraint.Expr, bool) {
	var expr constraint.Expr
	for _, cmt := range f.Comments {
		if len(cmt.List) == 0 {
			continue
		}
		for _, el := range cmt.List {
			if el.Pos() > f.Package {
				break
			}
			if line := el.Text; strings.HasPrefix(line, "//go:build") {
				var err error
				expr, err = constraint.Parse(line)
				if err != nil {
					expr = nil
				}
				break
			}
		}
	}

	name := pass.Fset.PositionFor(f.Pos(), false).Filename
	oexpr := constraintsFromName(name)
	if oexpr != nil {
		if expr == nil {
			expr = oexpr
		} else {
			expr = &constraint.AndExpr{X: expr, Y: oexpr}
		}
	}

	return expr, expr != nil
}

func constraintsFromName(name string) constraint.Expr {
	name = filepath.Base(name)
	name = strings.TrimSuffix(name, ".go")
	name = strings.TrimSuffix(name, "_test")
	var goos, goarch string
	switch strings.Count(name, "_") {
	case 0:
		// No GOOS or GOARCH in the file name.
	case 1:
		_, c, _ := strings.Cut(name, "_")
		if _, ok := knowledge.KnownGOOS[c]; ok {
			goos = c
		} else if _, ok := knowledge.KnownGOARCH[c]; ok {
			goarch = c
		}
	default:
		n := strings.LastIndex(name, "_")
		if _, ok := knowledge.KnownGOOS[name[n+1:]]; ok {
			// The file name is *_stuff_GOOS.go
			goos = name[n+1:]
		} else if _, ok := knowledge.KnownGOARCH[name[n+1:]]; ok {
			// The file name is *_GOOS_GOARCH.go or *_stuff_GOARCH.go
			goarch = name[n+1:]
			_, c, _ := strings.Cut(name[:n], "_")
			if _, ok := knowledge.KnownGOOS[c]; ok {
				// The file name is *_GOOS_GOARCH.go
				goos = c
			}
		} else {
			// The file name could also be something like foo_windows_nonsense.go — and because nonsense
			// isn't a known GOARCH, "windows" won't be interpreted as a GOOS, either.
		}
	}

	var expr constraint.Expr
	if goos != "" {
		expr = &constraint.TagExpr{Tag: goos}
	}
	if goarch != "" {
		if expr == nil {
			expr = &constraint.TagExpr{Tag: goarch}
		} else {
			expr = &constraint.AndExpr{X: expr, Y: &constraint.TagExpr{Tag: goarch}}
		}
	}
	return expr
}

// IsGenerated reports whether pos is in a generated file. It ignores
// //line directives.
func IsGenerated(pass *analysis.Pass, pos token.Pos) bool {
	_, ok := Generator(pass, pos)
	return ok
}

// Generator returns the generator that generated the file containing
// pos. It ignores //line directives.
func Generator(pass *analysis.Pass, pos token.Pos) (generated.Generator, bool) {
	file := pass.Fset.PositionFor(pos, false).Filename
	m := pass.ResultOf[generated.Analyzer].(map[string]generated.Generator)
	g, ok := m[file]
	return g, ok
}

// MayHaveSideEffects reports whether expr may have side effects. If
// the purity argument is nil, this function implements a purely
// syntactic check, meaning that any function call may have side
// effects, regardless of the called function's body. Otherwise,
// purity will be consulted to determine the purity of function calls.
func MayHaveSideEffects(pass *analysis.Pass, expr ast.Expr, purity purity.Result) bool {
	switch expr := expr.(type) {
	case *ast.BadExpr:
		return true
	case *ast.Ellipsis:
		return MayHaveSideEffects(pass, expr.Elt, purity)
	case *ast.FuncLit:
		// the literal itself cannot have side effects, only calling it
		// might, which is handled by CallExpr.
		return false
	case *ast.ArrayType, *ast.StructType, *ast.FuncType, *ast.InterfaceType, *ast.MapType, *ast.ChanType:
		// types cannot have side effects
		return false
	case *ast.BasicLit:
		return false
	case *ast.BinaryExpr:
		return MayHaveSideEffects(pass, expr.X, purity) || MayHaveSideEffects(pass, expr.Y, purity)
	case *ast.CallExpr:
		if purity == nil {
			return true
		}
		switch obj := typeutil.Callee(pass.TypesInfo, expr).(type) {
		case *types.Func:
			if _, ok := purity[obj]; !ok {
				return true
			}
		case *types.Builtin:
			switch obj.Name() {
			case "len", "cap":
			default:
				return true
			}
		default:
			return true
		}
		for _, arg := range expr.Args {
			if MayHaveSideEffects(pass, arg, purity) {
				return true
			}
		}
		return false
	case *ast.CompositeLit:
		if MayHaveSideEffects(pass, expr.Type, purity) {
			return true
		}
		for _, elt := range expr.Elts {
			if MayHaveSideEffects(pass, elt, purity) {
				return true
			}
		}
		return false
	case *ast.Ident:
		return false
	case *ast.IndexExpr:
		return MayHaveSideEffects(pass, expr.X, purity) || MayHaveSideEffects(pass, expr.Index, purity)
	case *ast.IndexListExpr:
		// In theory, none of the checks are necessary, as IndexListExpr only involves types. But there is no harm in
		// being safe.
		if MayHaveSideEffects(pass, expr.X, purity) {
			return true
		}
		for _, idx := range expr.Indices {
			if MayHaveSideEffects(pass, idx, purity) {
				return true
			}
		}
		return false
	case *ast.KeyValueExpr:
		return MayHaveSideEffects(pass, expr.Key, purity) || MayHaveSideEffects(pass, expr.Value, purity)
	case *ast.SelectorExpr:
		return MayHaveSideEffects(pass, expr.X, purity)
	case *ast.SliceExpr:
		return MayHaveSideEffects(pass, expr.X, purity) ||
			MayHaveSideEffects(pass, expr.Low, purity) ||
			MayHaveSideEffects(pass, expr.High, purity) ||
			MayHaveSideEffects(pass, expr.Max, purity)
	case *ast.StarExpr:
		return MayHaveSideEffects(pass, expr.X, purity)
	case *ast.TypeAssertExpr:
		return MayHaveSideEffects(pass, expr.X, purity)
	case *ast.UnaryExpr:
		if MayHaveSideEffects(pass, expr.X, purity) {
			return true
		}
		return expr.Op == token.ARROW || expr.Op == token.AND
	case *ast.ParenExpr:
		return MayHaveSideEffects(pass, expr.X, purity)
	case nil:
		return false
	default:
		panic(fmt.Sprintf("internal error: unhandled type %T", expr))
	}
}

// LanguageVersion returns the version of the Go language that node has access to. This
// might differ from the version of the Go standard library.
func LanguageVersion(pass *analysis.Pass, node Positioner) string {
	// As of Go 1.21, two places can specify the minimum Go version:
	// - 'go' directives in go.mod and go.work files
	// - individual files by using '//go:build'
	//
	// Individual files can upgrade to a higher version than the module version. Individual files
	// can also downgrade to a lower version, but only if the module version is at least Go 1.21.
	//
	// The restriction on downgrading doesn't matter to us. All language changes before Go 1.22 will
	// not type-check on versions that are too old, and thus never reach our analyzes. In practice,
	// such ineffective downgrading will always be useless, as the compiler will not restrict the
	// language features used, and doesn't ever rely on minimum versions to restrict the use of the
	// standard library. However, for us, both choices (respecting or ignoring ineffective
	// downgrading) have equal complexity, but only respecting it has a non-zero chance of reducing
	// noisy positives.
	//
	// The minimum Go versions are exposed via go/ast.File.GoVersion and go/types.Package.GoVersion.
	// ast.File's version is populated by the parser, whereas types.Package's version is populated
	// from the Go version specified in the types.Config, which is set by our package loader, based
	// on the module information provided by go/packages, via 'go list -json'.
	//
	// As of Go 1.21, standard library packages do not present themselves as modules, and thus do
	// not have a version set on their types.Package. In this case, we fall back to the version
	// provided by our '-go' flag. In most cases, '-go' defaults to 'module', which falls back to
	// the Go version that Staticcheck was built with when no module information exists. In the
	// future, the standard library will hopefully be a proper module (see
	// https://github.com/golang/go/issues/61174#issuecomment-1622471317). In that case, the version
	// of standard library packages will match that of the used Go version. At that point,
	// Staticcheck will refuse to work with Go versions that are too new, to avoid misinterpreting
	// code due to language changes.
	//
	// We also lack module information when building in GOPATH mode. In this case, the implied
	// language version is at most Go 1.21, as per https://github.com/golang/go/issues/60915. We
	// don't handle this yet, and it will not matter until Go 1.22.
	//
	// It is not clear how per-file downgrading behaves in GOPATH mode. On the one hand, no module
	// version at all is provided, which should preclude per-file downgrading. On the other hand,
	// https://github.com/golang/go/issues/60915 suggests that the language version is at most 1.21
	// in GOPATH mode, which would allow per-file downgrading. Again it doesn't affect us, as all
	// relevant language changes before Go 1.22 will lead to type-checking failures and never reach
	// us.
	//
	// Per-file upgrading is permitted in GOPATH mode.

	// If the file has its own Go version, we will return that. Otherwise, we default to
	// the type checker's GoVersion, which is populated from either the Go module, or from
	// our '-go' flag.
	return pass.TypesInfo.FileVersions[File(pass, node)]
}

// StdlibVersion returns the version of the Go standard library that node can expect to
// have access to. This might differ from the language version for versions of Go older
// than 1.21.
func StdlibVersion(pass *analysis.Pass, node Positioner) string {
	// The Go version as specified in go.mod or via the '-go' flag
	n := pass.Pkg.GoVersion()

	f := File(pass, node)
	if f == nil {
		panic(fmt.Sprintf("no file found for node with position %s", pass.Fset.PositionFor(node.Pos(), false)))
	}

	if nf := f.GoVersion; nf != "" {
		if version.Compare(n, "go1.21") == -1 {
			// Before Go 1.21, the Go version set in go.mod specified the maximum language
			// version available to the module. It wasn't uncommon to set the version to
			// Go 1.20 but restrict usage of 1.20 functionality (both language and stdlib)
			// to files tagged for 1.20, and supporting a lower version overall. As such,
			// a file tagged lower than the module version couldn't expect to have access
			// to the standard library of the version set in go.mod.
			//
			// At the same time, a file tagged higher than the module version, while not
			// able to use newer language features, would still have been able to use a
			// newer standard library.
			//
			// While Go 1.21's behavior has been backported to 1.19.11 and 1.20.6, users'
			// expectations have not.
			return nf
		} else {
			// Go 1.21 and newer refuse to build modules that depend on versions newer
			// than the used version of the Go toolchain. This means that in a 1.22 module
			// with a file tagged as 1.17, the file can expect to have access to 1.22's
			// standard library (but not to 1.22 language features). A file tagged with a
			// version higher than the minimum version has access to the newer standard
			// library (and language features.)
			//
			// Do note that strictly speaking we're conflating the Go version and the
			// module version in our check. Nothing is stopping a user from using Go 1.17
			// (which didn't implement the new rules for versions in go.mod) to build a Go
			// 1.22 module, in which case a file tagged with go1.17 will not have access to the 1.22
			// standard library. However, we believe that if a module requires 1.21 or
			// newer, then the author clearly expects the new behavior, and doesn't care
			// for the old one. Otherwise they would've specified an older version.
			//
			// In other words, the module version also specifies what it itself actually means, with
			// >=1.21 being a minimum version for the toolchain, and <1.21 being a maximum version for
			// the language.

			if version.Compare(nf, n) == 1 {
				return nf
			}
		}
	}

	return n
}

var integerLiteralQ = pattern.MustParse(`(IntegerLiteral tv)`)

func IntegerLiteral(pass *analysis.Pass, node ast.Node) (types.TypeAndValue, bool) {
	m, ok := Match(pass, integerLiteralQ, node)
	if !ok {
		return types.TypeAndValue{}, false
	}
	return m.State["tv"].(types.TypeAndValue), true
}

func IsIntegerLiteral(pass *analysis.Pass, node ast.Node, value constant.Value) bool {
	tv, ok := IntegerLiteral(pass, node)
	if !ok {
		return false
	}
	return constant.Compare(tv.Value, token.EQL, value)
}

// IsMethod reports whether expr is a method call of a named method with signature meth.
// If name is empty, it is not checked.
// For now, method expressions (Type.Method(recv, ..)) are not considered method calls.
func IsMethod(pass *analysis.Pass, expr *ast.SelectorExpr, name string, meth *types.Signature) bool {
	if name != "" && expr.Sel.Name != name {
		return false
	}
	sel, ok := pass.TypesInfo.Selections[expr]
	if !ok || sel.Kind() != types.MethodVal {
		return false
	}
	return types.Identical(sel.Type(), meth)
}

func RefersTo(pass *analysis.Pass, expr ast.Expr, ident types.Object) bool {
	found := false
	fn := func(node ast.Node) bool {
		ident2, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if ident == pass.TypesInfo.ObjectOf(ident2) {
			found = true
			return false
		}
		return true
	}
	ast.Inspect(expr, fn)
	return found
}
