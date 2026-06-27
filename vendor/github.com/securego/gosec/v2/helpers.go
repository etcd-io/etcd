// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gosec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrUnexpectedASTNode     = errors.New("unexpected AST node type")
	ErrNoProjectRelativePath = errors.New("no project relative path found")
	ErrNoProjectAbsolutePath = errors.New("no project absolute path found")
)

// envGoModVersion overrides the Go version detection.
const envGoModVersion = "GOSECGOVERSION"

// MatchCallByPackage ensures that the specified package is imported,
// adjusts the name for any aliases and ignores cases that are
// initialization only imports.
//
// Usage:
//
//	node, matched := MatchCallByPackage(n, ctx, "math/rand", "Read")
func MatchCallByPackage(n ast.Node, c *Context, pkg string, names ...string) (*ast.CallExpr, bool) {
	importedNames, found := GetImportedNames(pkg, c)
	if !found {
		return nil, false
	}

	if callExpr, ok := n.(*ast.CallExpr); ok {
		packageName, callName, err := GetCallInfo(callExpr, c)
		if err != nil {
			return nil, false
		}
		for _, in := range importedNames {
			if packageName != in {
				continue
			}
			for _, name := range names {
				if callName == name {
					return callExpr, true
				}
			}
		}
	}
	return nil, false
}

// MatchCompLit will match an ast.CompositeLit based on the supplied type
func MatchCompLit(n ast.Node, ctx *Context, required string) *ast.CompositeLit {
	if complit, ok := n.(*ast.CompositeLit); ok {
		typeOf := ctx.Info.TypeOf(complit)
		if typeOf.String() == required {
			return complit
		}
	}
	return nil
}

// GetInt will read and return an integer value from an ast.BasicLit
func GetInt(n ast.Node) (int64, error) {
	if node, ok := n.(*ast.BasicLit); ok && node.Kind == token.INT {
		return strconv.ParseInt(node.Value, 0, 64)
	}
	return 0, fmt.Errorf("%w: %T", ErrUnexpectedASTNode, n)
}

// GetFloat will read and return a float value from an ast.BasicLit
func GetFloat(n ast.Node) (float64, error) {
	if node, ok := n.(*ast.BasicLit); ok && node.Kind == token.FLOAT {
		return strconv.ParseFloat(node.Value, 64)
	}
	return 0.0, fmt.Errorf("%w: %T", ErrUnexpectedASTNode, n)
}

// GetChar will read and return a char value from an ast.BasicLit
func GetChar(n ast.Node) (byte, error) {
	if node, ok := n.(*ast.BasicLit); ok && node.Kind == token.CHAR {
		return node.Value[0], nil
	}
	return 0, fmt.Errorf("%w: %T", ErrUnexpectedASTNode, n)
}

// GetStringRecursive will recursively walk down a tree of *ast.BinaryExpr. It will then concat the results, and return.
// Unlike the other getters, it does _not_ raise an error for unknown ast.Node types. At the base, the recursion will hit a non-BinaryExpr type,
// either BasicLit or other, so it's not an error case. It will only error if `strconv.Unquote` errors. This matters, because there's
// currently functionality that relies on error values being returned by GetString if and when it hits a non-basiclit string node type,
// hence for cases where recursion is needed, we use this separate function, so that we can still be backwards compatible.
//
// This was added to handle a SQL injection concatenation case where the injected value is infixed between two strings, not at the start or end. See example below
//
// Do note that this will omit non-string values. So for example, if you were to use this node:
// ```go
// q := "SELECT * FROM foo WHERE name = '" + os.Args[0] + "' AND 1=1" // will result in "SELECT * FROM foo WHERE ” AND 1=1"

func GetStringRecursive(n ast.Node) (string, error) {
	if node, ok := n.(*ast.BasicLit); ok && node.Kind == token.STRING {
		return strconv.Unquote(node.Value)
	}

	if expr, ok := n.(*ast.BinaryExpr); ok {
		x, err := GetStringRecursive(expr.X)
		if err != nil {
			return "", err
		}

		y, err := GetStringRecursive(expr.Y)
		if err != nil {
			return "", err
		}

		return x + y, nil
	}

	return "", nil
}

// GetString will read and return a string value from an ast.BasicLit
func GetString(n ast.Node) (string, error) {
	if node, ok := n.(*ast.BasicLit); ok && node.Kind == token.STRING {
		return strconv.Unquote(node.Value)
	}

	return "", fmt.Errorf("%w: %T", ErrUnexpectedASTNode, n)
}

// GetCallObject returns the object and call expression and associated
// object for a given AST node. nil, nil will be returned if the
// object cannot be resolved.
func GetCallObject(n ast.Node, ctx *Context) (*ast.CallExpr, types.Object) {
	switch node := n.(type) {
	case *ast.CallExpr:
		switch fn := node.Fun.(type) {
		case *ast.Ident:
			return node, ctx.Info.Uses[fn]
		case *ast.SelectorExpr:
			return node, ctx.Info.Uses[fn.Sel]
		}
	}
	return nil, nil
}

type callInfo struct {
	packageName string
	funcName    string
	err         error
}

var callCachePool = sync.Pool{
	New: func() any {
		return make(map[ast.Node]callInfo)
	},
}

// GetCallInfo returns the package or type and name  associated with a
// call expression.
func GetCallInfo(n ast.Node, ctx *Context) (string, string, error) {
	if ctx.callCache != nil {
		if res, ok := ctx.callCache[n]; ok {
			return res.packageName, res.funcName, res.err
		}
	}

	packageName, funcName, err := getCallInfo(n, ctx)
	if ctx.callCache != nil {
		ctx.callCache[n] = callInfo{packageName, funcName, err}
	}
	return packageName, funcName, err
}

func getCallInfo(n ast.Node, ctx *Context) (string, string, error) {
	switch node := n.(type) {
	case *ast.CallExpr:
		switch fn := node.Fun.(type) {
		case *ast.SelectorExpr:
			switch expr := fn.X.(type) {
			case *ast.Ident:
				if expr.Obj != nil && expr.Obj.Kind == ast.Var {
					t := ctx.Info.TypeOf(expr)
					if t != nil {
						return t.String(), fn.Sel.Name, nil
					}
					return "undefined", fn.Sel.Name, fmt.Errorf("missing type info")
				}
				return expr.Name, fn.Sel.Name, nil
			case *ast.SelectorExpr:
				if expr.Sel != nil {
					t := ctx.Info.TypeOf(expr.Sel)
					if t != nil {
						return t.String(), fn.Sel.Name, nil
					}
					return "undefined", fn.Sel.Name, fmt.Errorf("missing type info")
				}
			case *ast.CallExpr:
				switch call := expr.Fun.(type) {
				case *ast.Ident:
					if call.Name == "new" && len(expr.Args) > 0 {
						t := ctx.Info.TypeOf(expr.Args[0])
						if t != nil {
							return t.String(), fn.Sel.Name, nil
						}
						return "undefined", fn.Sel.Name, fmt.Errorf("missing type info")
					}
					if call.Obj != nil {
						switch decl := call.Obj.Decl.(type) {
						case *ast.FuncDecl:
							ret := decl.Type.Results
							if ret != nil && len(ret.List) > 0 {
								ret1 := ret.List[0]
								if ret1 != nil {
									t := ctx.Info.TypeOf(ret1.Type)
									if t != nil {
										return t.String(), fn.Sel.Name, nil
									}
									return "undefined", fn.Sel.Name, fmt.Errorf("missing type info")
								}
							}
						}
					}
				}
			}
		case *ast.Ident:
			return ctx.Pkg.Name(), fn.Name, nil
		}
	}

	return "", "", fmt.Errorf("unable to determine call info")
}

// GetCallStringArgsValues returns the values of strings arguments if they can be resolved
func GetCallStringArgsValues(n ast.Node, _ *Context) []string {
	values := []string{}
	switch node := n.(type) {
	case *ast.CallExpr:
		for _, arg := range node.Args {
			switch param := arg.(type) {
			case *ast.BasicLit:
				value, err := GetString(param)
				if err == nil {
					values = append(values, value)
				}
			case *ast.Ident:
				values = append(values, GetIdentStringValues(param)...)
			}
		}
	}
	return values
}

func getIdentStringValues(ident *ast.Ident, stringFinder func(ast.Node) (string, error)) []string {
	values := []string{}
	obj := ident.Obj
	if obj != nil {
		switch decl := obj.Decl.(type) {
		case *ast.ValueSpec:
			for _, v := range decl.Values {
				value, err := stringFinder(v)
				if err == nil {
					values = append(values, value)
				}
			}
		case *ast.AssignStmt:
			for _, v := range decl.Rhs {
				value, err := stringFinder(v)
				if err == nil {
					values = append(values, value)
				}
			}
		}
	}
	return values
}

// GetIdentStringValuesRecursive returns the string of values of an Ident if they can be resolved
// The difference between this and GetIdentStringValues is that it will attempt to resolve the strings recursively,
// if it is passed a *ast.BinaryExpr. See GetStringRecursive for details
func GetIdentStringValuesRecursive(ident *ast.Ident) []string {
	return getIdentStringValues(ident, GetStringRecursive)
}

// GetIdentStringValues return the string values of an Ident if they can be resolved
func GetIdentStringValues(ident *ast.Ident) []string {
	return getIdentStringValues(ident, GetString)
}

// GetBinaryExprOperands returns all operands of a binary expression by traversing
// the expression tree
func GetBinaryExprOperands(be *ast.BinaryExpr) []ast.Node {
	var traverse func(be *ast.BinaryExpr)
	result := []ast.Node{}
	traverse = func(be *ast.BinaryExpr) {
		if lhs, ok := be.X.(*ast.BinaryExpr); ok {
			traverse(lhs)
		} else {
			result = append(result, be.X)
		}
		if rhs, ok := be.Y.(*ast.BinaryExpr); ok {
			traverse(rhs)
		} else {
			result = append(result, be.Y)
		}
	}
	traverse(be)
	return result
}

// GetImportedNames returns the name(s)/alias(es) used for the package within
// the code. It ignores initialization-only imports.
func GetImportedNames(path string, ctx *Context) (names []string, found bool) {
	importNames, imported := ctx.Imports.Imported[path]
	return importNames, imported
}

// GetImportPath resolves the full import path of an identifier based on
// the imports in the current context(including aliases).
func GetImportPath(name string, ctx *Context) (string, bool) {
	for path := range ctx.Imports.Imported {
		if imported, ok := GetImportedNames(path, ctx); ok {
			for _, n := range imported {
				if n == name {
					return path, true
				}
			}
		}
	}

	return "", false
}

// GetLocation returns the filename and line number of an ast.Node
func GetLocation(n ast.Node, ctx *Context) (string, int) {
	fobj := ctx.FileSet.File(n.Pos())
	return fobj.Name(), fobj.Line(n.Pos())
}

// Gopath returns all GOPATHs
func Gopath() []string {
	defaultGoPath := runtime.GOROOT()
	if u, err := user.Current(); err == nil {
		defaultGoPath = filepath.Join(u.HomeDir, "go")
	}
	path := Getenv("GOPATH", defaultGoPath)
	paths := strings.Split(path, string(os.PathListSeparator))
	for idx, path := range paths {
		if abs, err := filepath.Abs(path); err == nil {
			paths[idx] = abs
		}
	}
	return paths
}

// Getenv returns the values of the environment variable, otherwise
// returns the default if variable is not set
func Getenv(key, userDefault string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return userDefault
}

// GetPkgRelativePath returns the Go relative path derived
// form the given path
func GetPkgRelativePath(path string) (string, error) {
	abspath, err := filepath.Abs(path)
	if err != nil {
		abspath = path
	}
	if strings.HasSuffix(abspath, ".go") {
		abspath = filepath.Dir(abspath)
	}
	for _, base := range Gopath() {
		projectRoot := filepath.FromSlash(fmt.Sprintf("%s/src/", base))
		if strings.HasPrefix(abspath, projectRoot) {
			return strings.TrimPrefix(abspath, projectRoot), nil
		}
	}
	return "", ErrNoProjectRelativePath
}

// GetPkgAbsPath returns the Go package absolute path derived from
// the given path
func GetPkgAbsPath(pkgPath string) (string, error) {
	absPath, err := filepath.Abs(pkgPath)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", ErrNoProjectAbsolutePath
	}
	return absPath, nil
}

// ConcatString recursively concatenates constant strings from an expression
// if the entire chain is fully constant-derived (using TryResolve).
// Returns the concatenated string and true if successful.
func ConcatString(expr ast.Expr, ctx *Context) (string, bool) {
	if expr == nil || !TryResolve(expr, ctx) {
		return "", false
	}

	var build strings.Builder
	var traverse func(ast.Expr) bool
	traverse = func(e ast.Expr) bool {
		switch node := e.(type) {
		case *ast.BasicLit:
			if str, err := GetString(node); err == nil {
				build.WriteString(str)
				return true
			}
			return false
		case *ast.Ident:
			values := GetIdentStringValuesRecursive(node)
			for _, v := range values {
				build.WriteString(v)
			}
			return len(values) > 0
		case *ast.BinaryExpr:
			if node.Op != token.ADD {
				return false
			}
			return traverse(node.X) && traverse(node.Y)
		default:
			return false
		}
	}

	if traverse(expr) {
		return build.String(), true
	}
	return "", false
}

// FindVarIdentities returns array of all variable identities in a given binary expression
func FindVarIdentities(n *ast.BinaryExpr, c *Context) ([]*ast.Ident, bool) {
	identities := []*ast.Ident{}
	// sub expressions are found in X object, Y object is always the last term
	if rightOperand, ok := n.Y.(*ast.Ident); ok {
		obj := c.Info.ObjectOf(rightOperand)
		if _, ok := obj.(*types.Var); ok && !TryResolve(rightOperand, c) {
			identities = append(identities, rightOperand)
		}
	}
	if leftOperand, ok := n.X.(*ast.BinaryExpr); ok {
		if leftIdentities, ok := FindVarIdentities(leftOperand, c); ok {
			identities = append(identities, leftIdentities...)
		}
	} else {
		if leftOperand, ok := n.X.(*ast.Ident); ok {
			obj := c.Info.ObjectOf(leftOperand)
			if _, ok := obj.(*types.Var); ok && !TryResolve(leftOperand, c) {
				identities = append(identities, leftOperand)
			}
		}
	}

	if len(identities) > 0 {
		return identities, true
	}
	// if nil or error, return false
	return nil, false
}

// FindModuleRoot returns the directory containing the go.mod file that
// governs the given directory. It walks upward from dir until it finds
// a go.mod file or reaches the filesystem root.
// Returns "" if no go.mod is found.
//
// This is needed to correctly load packages in multi-module repositories:
// without setting packages.Config.Dir to the module root, packages.Load
// uses the current working directory for module resolution, which fails
// when the CWD belongs to a different module than the package being loaded.
func FindModuleRoot(dir string) string {
	dir = filepath.Clean(dir)
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return ""
		}
		dir = parent
	}
}

// PackagePaths returns a slice with all packages path at given root directory
func PackagePaths(root string, excludes []*regexp.Regexp) ([]string, error) {
	if strings.HasSuffix(root, "...") {
		root = root[0 : len(root)-3]
	} else {
		return []string{root}, nil
	}
	paths := map[string]bool{}
	err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if filepath.Ext(path) == ".go" {
			path = filepath.Dir(path)
			if isExcluded(filepath.ToSlash(path), excludes) {
				return nil
			}
			paths[path] = true
		}
		return nil
	})
	if err != nil {
		return []string{}, err
	}

	result := []string{}
	for path := range paths {
		result = append(result, path)
	}
	return result, nil
}

// isExcluded checks if a string matches any of the exclusion regexps
func isExcluded(str string, excludes []*regexp.Regexp) bool {
	if excludes == nil {
		return false
	}
	for _, exclude := range excludes {
		if exclude != nil && exclude.MatchString(str) {
			return true
		}
	}
	return false
}

// ExcludedDirsRegExp builds the regexps for a list of excluded dirs provided as strings
func ExcludedDirsRegExp(excludedDirs []string) []*regexp.Regexp {
	var exps []*regexp.Regexp
	for _, excludedDir := range excludedDirs {
		str := fmt.Sprintf(`([\\/])?%s([\\/])?`, strings.ReplaceAll(filepath.ToSlash(excludedDir), "/", `\/`))
		r := regexp.MustCompile(str)
		exps = append(exps, r)
	}
	return exps
}

// RootPath returns the absolute root path of a scan
func RootPath(root string) (string, error) {
	root = strings.TrimSuffix(root, "...")
	return filepath.Abs(root)
}

var (
	goVersionCache struct {
		major, minor, build int
	}
	goVersionOnce sync.Once
)

// GoVersion returns parsed version of Go mod version and fallback to runtime version if not found.
func GoVersion() (int, int, int) {
	goVersionOnce.Do(func() {
		if env, ok := os.LookupEnv(envGoModVersion); ok {
			goVersionCache.major, goVersionCache.minor, goVersionCache.build = parseGoVersion(strings.TrimPrefix(env, "go"))
			return
		}

		goVersion, err := goModVersion()
		if err != nil {
			goVersionCache.major, goVersionCache.minor, goVersionCache.build = parseGoVersion(strings.TrimPrefix(runtime.Version(), "go"))
			return
		}

		goVersionCache.major, goVersionCache.minor, goVersionCache.build = parseGoVersion(goVersion)
	})
	return goVersionCache.major, goVersionCache.minor, goVersionCache.build
}

type goListOutput struct {
	GoVersion string `json:"GoVersion"`
}

func goModVersion() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-json")

	raw, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command go list: %w: %s", err, string(raw))
	}

	var v goListOutput
	err = json.NewDecoder(bytes.NewBuffer(raw)).Decode(&v)
	if err != nil {
		return "", fmt.Errorf("unmarshaling error: %w: %s", err, string(raw))
	}

	return v.GoVersion, nil
}

// parseGoVersion parses Go version.
// example:
// - 1.19rc2
// - 1.19beta2
// - 1.19.4
// - 1.19
func parseGoVersion(version string) (int, int, int) {
	exp := regexp.MustCompile(`(\d+).(\d+)(?:.(\d+))?.*`)
	parts := exp.FindStringSubmatch(version)
	if len(parts) <= 1 {
		return 0, 0, 0
	}

	major, _ := strconv.Atoi(parts[1])
	minor, _ := strconv.Atoi(parts[2])
	build, _ := strconv.Atoi(parts[3])

	return major, minor, build
}

// CLIBuildTags converts a list of Go build tags into the corresponding CLI
// build flag (-tags=form) by trimming whitespace, removing empty entries,
// and joining them into a comma-separated -tags argument for use with go build
// commands.
func CLIBuildTags(buildTags []string) []string {
	var buildFlags []string
	if len(buildTags) > 0 {
		for _, tag := range buildTags {
			// remove empty entries and surrounding whitespace
			if t := strings.TrimSpace(tag); t != "" {
				buildFlags = append(buildFlags, t)
			}
		}
		if len(buildFlags) > 0 {
			buildFlags = []string{"-tags=" + strings.Join(buildFlags, ",")}
		}
	}

	return buildFlags
}

// ContainingFile returns the *ast.File from ctx.PkgFiles that contains the given position provider.
// A position provider can be an ast.Node, a types.Object, or any type with a Pos() token.Pos method.
// Returns nil if not found or if the provider is nil/invalid.
func ContainingFile(p interface{ Pos() token.Pos }, ctx *Context) *ast.File {
	if p == nil {
		return nil
	}
	pos := p.Pos()
	if !pos.IsValid() {
		return nil
	}
	for _, f := range ctx.PkgFiles {
		if f.Pos() <= pos && pos < f.End() {
			return f
		}
	}
	return nil
}
