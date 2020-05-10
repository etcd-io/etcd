package suggest

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mdempsky/gocode/internal/lookdot"
)

type Config struct {
	Importer           types.Importer
	Logf               func(fmt string, args ...interface{})
	Builtin            bool
	IgnoreCase         bool
	UnimportedPackages bool
}

var cache = struct {
	lock  sync.Mutex
	files map[string]fileCacheEntry
	fset  *token.FileSet
}{
	files: make(map[string]fileCacheEntry),
	fset:  token.NewFileSet(),
}

type fileCacheEntry struct {
	file  *ast.File
	mtime time.Time
}

// Suggest returns a list of suggestion candidates and the length of
// the text that should be replaced, if any.
func (c *Config) Suggest(filename string, data []byte, cursor int) ([]Candidate, int) {
	if cursor < 0 {
		return nil, 0
	}

	fset, pos, pkg, imports := c.analyzePackage(filename, data, cursor)
	if pkg == nil {
		c.Logf("no package found for %s", filename)
		return nil, 0
	}
	scope := pkg.Scope().Innermost(pos)

	ctx, expr, partial := deduceCursorContext(data, cursor)
	b := candidateCollector{
		localpkg:   pkg,
		imports:    imports,
		partial:    partial,
		filter:     objectFilters[partial],
		builtin:    ctx != selectContext && c.Builtin,
		ignoreCase: c.IgnoreCase,
	}

	switch ctx {
	case emptyResultsContext:
		// don't show results in certain cases
		return nil, 0

	case selectContext:
		tv, _ := types.Eval(fset, pkg, pos, expr)
		if lookdot.Walk(&tv, b.appendObject) {
			break
		}

		_, obj := scope.LookupParent(expr, pos)
		if pkgName, isPkg := obj.(*types.PkgName); isPkg {
			c.packageCandidates(pkgName.Imported(), &b)
			break
		}
		if !c.UnimportedPackages {
			return nil, 0
		}
		pkg := c.resolveKnownPackageIdent(expr)
		if pkg == nil {
			return nil, 0
		}
		c.packageCandidates(pkg, &b)

	case compositeLiteralContext:
		tv, _ := types.Eval(fset, pkg, pos, expr)
		if tv.IsType() {
			if _, isStruct := tv.Type.Underlying().(*types.Struct); isStruct {
				c.fieldNameCandidates(tv.Type, &b)
				break
			}
		}
		fallthrough
	case unknownContext:
		c.scopeCandidates(scope, pos, &b)
	}

	res := b.getCandidates()
	if len(res) == 0 {
		return nil, 0
	}
	return res, len(partial)
}

func (c *Config) parseOtherFile(filename string) *ast.File {
	entry := cache.files[filename]

	fi, err := os.Stat(filename)
	if err != nil {
		// TODO(mdempsky): How to handle this cleanly?
		panic(err)
	}

	if entry.mtime != fi.ModTime() {
		file, err := parser.ParseFile(cache.fset, filename, nil, 0)
		if err != nil {
			c.logParseError(fmt.Sprintf("Error parsing %q", filename), err)
		}
		trimAST(file, token.NoPos)

		entry = fileCacheEntry{file, fi.ModTime()}
		cache.files[filename] = entry
	}

	return entry.file
}

func (c *Config) analyzePackage(filename string, data []byte, cursor int) (*token.FileSet, token.Pos, *types.Package, []*ast.ImportSpec) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	// Reset every 1GB of files so fset doesn't overflow.
	if cache.fset.Base() >= 1e9 {
		cache.fset = token.NewFileSet()
		cache.files = make(map[string]fileCacheEntry)
	}

	// Delete random files to keep the cache at most 100 entries.
	for k := range cache.files {
		if len(cache.files) <= 100 {
			break
		}
		delete(cache.files, k)
	}

	// If we're in trailing white space at the end of a scope,
	// sometimes go/types doesn't recognize that variables should
	// still be in scope there.
	filesemi := bytes.Join([][]byte{data[:cursor], []byte(";"), data[cursor:]}, nil)

	fileAST, err := parser.ParseFile(cache.fset, filename, filesemi, parser.AllErrors)
	if err != nil {
		c.logParseError("Error parsing input file (outer block)", err)
	}
	astPos := fileAST.Pos()
	if astPos == 0 {
		return nil, token.NoPos, nil, nil
	}
	pos := cache.fset.File(astPos).Pos(cursor)
	trimAST(fileAST, pos)

	files := []*ast.File{fileAST}
	for _, otherName := range c.findOtherPackageFiles(filename, fileAST.Name.Name) {
		files = append(files, c.parseOtherFile(otherName))
	}

	cfg := types.Config{
		Importer: c.Importer,
		Error:    func(err error) {},
	}
	pkg, _ := cfg.Check("", cache.fset, files, nil)

	return cache.fset, pos, pkg, fileAST.Imports
}

// trimAST clears any part of the AST not relevant to type checking
// expressions at pos.
func trimAST(file *ast.File, pos token.Pos) {
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if pos < n.Pos() || pos >= n.End() {
			switch n := n.(type) {
			case *ast.FuncDecl:
				n.Body = nil
			case *ast.BlockStmt:
				n.List = nil
			case *ast.CaseClause:
				n.Body = nil
			case *ast.CommClause:
				n.Body = nil
			case *ast.CompositeLit:
				// Leave elts in place for [...]T
				// array literals, because they can
				// affect the expression's type.
				if !isEllipsisArray(n.Type) {
					n.Elts = nil
				}
			}
		}
		return true
	})
}

func isEllipsisArray(n ast.Expr) bool {
	at, ok := n.(*ast.ArrayType)
	if !ok {
		return false
	}
	_, ok = at.Len.(*ast.Ellipsis)
	return ok
}

func (c *Config) fieldNameCandidates(typ types.Type, b *candidateCollector) {
	s := typ.Underlying().(*types.Struct)
	for i, n := 0, s.NumFields(); i < n; i++ {
		b.appendObject(s.Field(i))
	}
}

func (c *Config) packageCandidates(pkg *types.Package, b *candidateCollector) {
	c.scopeCandidates(pkg.Scope(), token.NoPos, b)
}

func (c *Config) scopeCandidates(scope *types.Scope, pos token.Pos, b *candidateCollector) {
	seen := make(map[string]bool)
	for scope != nil {
		for _, name := range scope.Names() {
			if seen[name] {
				continue
			}
			seen[name] = true
			_, obj := scope.LookupParent(name, pos)
			if obj != nil {
				b.appendObject(obj)
			}
		}
		scope = scope.Parent()
	}
}

func (c *Config) logParseError(intro string, err error) {
	if c.Logf == nil {
		return
	}
	if el, ok := err.(scanner.ErrorList); ok {
		c.Logf("%s:", intro)
		for _, er := range el {
			c.Logf(" %s", er)
		}
	} else {
		c.Logf("%s: %s", intro, err)
	}
}

func (c *Config) findOtherPackageFiles(filename, pkgName string) []string {
	if filename == "" {
		return nil
	}

	dir, file := filepath.Split(filename)
	dents, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	isTestFile := strings.HasSuffix(file, "_test.go")

	// TODO(mdempsky): Use go/build.(*Context).MatchFile or
	// something to properly handle build tags?
	var out []string
	for _, dent := range dents {
		name := dent.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		if name == file || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !isTestFile && strings.HasSuffix(name, "_test.go") {
			continue
		}

		abspath := filepath.Join(dir, name)
		if pkgNameFor(abspath) == pkgName {
			out = append(out, abspath)
		}
	}

	return out
}

func (c *Config) resolveKnownPackageIdent(pkgName string) *types.Package {
	pkgName, ok := knownPackageIdents[pkgName]
	if !ok {
		return nil
	}
	pkg, _ := c.Importer.Import(pkgName)
	return pkg
}

func pkgNameFor(filename string) string {
	file, _ := parser.ParseFile(token.NewFileSet(), filename, nil, parser.PackageClauseOnly)
	if file == nil {
		return ""
	}
	return file.Name.Name
}
