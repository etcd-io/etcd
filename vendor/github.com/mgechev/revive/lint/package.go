package lint

import (
	"errors"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"sync"

	goversion "github.com/hashicorp/go-version"
	"golang.org/x/sync/errgroup"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/internal/typeparams"
)

// Package represents a package in the project.
type Package struct {
	fset *token.FileSet

	mu        sync.RWMutex
	files     map[string]*File
	goVersion *goversion.Version
	typesPkg  *types.Package
	typesInfo *types.Info
	// sortable is the set of types in the package that implement sort.Interface.
	sortable map[string]bool
	// main is whether this is a "main" package.
	main int
}

var (
	trueValue  = 1
	falseValue = 2

	// Go115 is a constant representing the Go version 1.15.
	Go115 = goversion.Must(goversion.NewVersion("1.15"))
	// Go121 is a constant representing the Go version 1.21.
	Go121 = goversion.Must(goversion.NewVersion("1.21"))
	// Go122 is a constant representing the Go version 1.22.
	Go122 = goversion.Must(goversion.NewVersion("1.22"))
	// Go124 is a constant representing the Go version 1.24.
	Go124 = goversion.Must(goversion.NewVersion("1.24"))
	// Go125 is a constant representing the Go version 1.25.
	Go125 = goversion.Must(goversion.NewVersion("1.25"))
)

// Files return package's files.
func (p *Package) Files() map[string]*File {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.files
}

// IsMain returns if that's the main package.
func (p *Package) IsMain() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch p.main {
	case trueValue:
		return true
	case falseValue:
		return false
	}
	for _, f := range p.files {
		if f.isMain() {
			p.main = trueValue
			return true
		}
	}
	p.main = falseValue
	return false
}

// TypesPkg yields information on this package.
func (p *Package) TypesPkg() *types.Package {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.typesPkg
}

// TypesInfo yields type information of this package identifiers.
func (p *Package) TypesInfo() *types.Info {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.typesInfo
}

// Sortable yields a map of sortable types in this package.
func (p *Package) Sortable() map[string]bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.sortable
}

// TypeCheck performs type checking for given package.
func (p *Package) TypeCheck() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	alreadyTypeChecked := p.typesInfo != nil || p.typesPkg != nil
	if alreadyTypeChecked {
		return nil
	}

	config := &types.Config{
		// By setting a no-op error reporter, the type checker does as much work as possible.
		Error:    func(error) {},
		Importer: importer.Default(),
	}
	info := &types.Info{
		Types:  map[ast.Expr]types.TypeAndValue{},
		Defs:   map[*ast.Ident]types.Object{},
		Uses:   map[*ast.Ident]types.Object{},
		Scopes: map[ast.Node]*types.Scope{},
	}
	var anyFile *File
	var astFiles []*ast.File
	for _, f := range p.files {
		anyFile = f
		astFiles = append(astFiles, f.AST)
	}

	if anyFile == nil {
		// this is unlikely to happen, but technically guarantees anyFile to not be nil
		return errors.New("no ast.File found")
	}

	typesPkg, err := check(config, anyFile.AST.Name.Name, p.fset, astFiles, info)

	// Remember the typechecking info, even if config.Check failed,
	// since we will get partial information.
	p.typesPkg = typesPkg
	p.typesInfo = info

	return err
}

// check function encapsulates the call to [go/types.Config.Check] method and
// recovers if the called method panics (see issue #59).
func check(config *types.Config, n string, fset *token.FileSet, astFiles []*ast.File, info *types.Info) (p *types.Package, err error) {
	defer func() {
		if r := recover(); r != nil {
			err, _ = r.(error)
			p = nil
			return
		}
	}()

	return config.Check(n, fset, astFiles, info)
}

// TypeOf returns the type of expression.
func (p *Package) TypeOf(expr ast.Expr) types.Type {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.typesInfo == nil {
		return nil
	}

	return p.typesInfo.TypeOf(expr)
}

type sortableMethodsFlags int

// flags for sortable interface methods.
const (
	bfLen sortableMethodsFlags = 1 << iota
	bfLess
	bfSwap
)

func (p *Package) scanSortable() {
	p.mu.Lock()
	defer p.mu.Unlock()

	sortableFlags := map[string]sortableMethodsFlags{}
	for _, f := range p.files {
		for _, decl := range f.AST.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			isAMethodDeclaration := ok && fn.Recv != nil && len(fn.Recv.List) != 0
			if !isAMethodDeclaration {
				continue
			}

			recvType := typeparams.ReceiverType(fn)
			sortableFlags[recvType] |= getSortableMethodFlagForFunction(fn)
		}
	}

	p.sortable = make(map[string]bool, len(sortableFlags))
	for typ, ms := range sortableFlags {
		if ms == bfLen|bfLess|bfSwap {
			p.sortable[typ] = true
		}
	}
}

func (p *Package) lint(rules []Rule, config Config, failures chan Failure) error {
	p.scanSortable()
	var eg errgroup.Group
	for _, file := range p.Files() {
		eg.Go(func() error {
			return file.lint(rules, config, failures)
		})
	}

	return eg.Wait()
}

// IsAtLeastGoVersion returns true if the Go version for this package is v or higher, false otherwise.
func (p *Package) IsAtLeastGoVersion(v *goversion.Version) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.goVersion.GreaterThanOrEqual(v)
}

func getSortableMethodFlagForFunction(fn *ast.FuncDecl) sortableMethodsFlags {
	switch {
	case astutils.FuncSignatureIs(fn, "Len", []string{}, []string{"int"}):
		return bfLen
	case astutils.FuncSignatureIs(fn, "Less", []string{"int", "int"}, []string{"bool"}):
		return bfLess
	case astutils.FuncSignatureIs(fn, "Swap", []string{"int", "int"}, []string{}):
		return bfSwap
	default:
		return 0
	}
}
