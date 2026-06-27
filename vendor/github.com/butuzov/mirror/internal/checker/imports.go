package checker

import (
	"go/ast"
	"go/token"
	"path"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/ast/inspector"
)

// Imports represents an imported package in a nice for lookup way...
//
//	examples:
//	import . "bytes"    -> checker.Import{Pkg:"bytes", Val:"."}
//	import name "bytes" -> checker.Import{Pkg:"bytes", Val:"name"}
type Import struct {
	Pkg  string // package name
	Name string // alias
}

type Imports map[string][]Import

// we are going to have Imports entries to be sorted, but if it has less then
// `sortLowerLimit` elements we are skipping this step as its not going to
// be worth of effort.
const sortLowerLimit int = 13

// Package level lock is to prevent import map corruption
var lock sync.RWMutex

func Load(fs *token.FileSet, ins *inspector.Inspector) Imports {
	lock.Lock()
	defer lock.Unlock()

	imports := make(Imports)

	// Populate imports map
	ins.Preorder([]ast.Node{(*ast.ImportSpec)(nil)}, func(node ast.Node) {
		importSpec, _ := node.(*ast.ImportSpec)

		var (
			key  = fs.Position(node.Pos()).Filename
			pkg  = strings.Trim(importSpec.Path.Value, `"`)
			name = importSpec.Name.String()
		)

		if importSpec.Name == nil {
			name = path.Base(pkg) // note: we need only basename of the package
		}

		imports[key] = append(imports[key], Import{
			Pkg:  pkg,
			Name: name,
		})
	})

	imports.sort()

	return imports
}

// sort will sort imports for each of the checking files.
func (i *Imports) sort() {
	for k := range *i {
		if len((*i)[k]) < sortLowerLimit {
			continue
		}

		k := k
		sort.Slice((*i)[k], func(left, right int) bool {
			return (*i)[k][left].Name < (*i)[k][right].Name
		})
	}
}

func (i Imports) Lookup(file, pkg string) (string, bool) {
	if _, ok := i[file]; ok {
		for idx := range i[file] {
			if i[file][idx].Name == pkg {
				return i[file][idx].Pkg, true
			}
		}
	}

	return "", false
}
