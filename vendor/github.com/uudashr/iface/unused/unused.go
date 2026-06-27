package unused

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"slices"
	"strings"

	"github.com/uudashr/iface/internal/directive"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer detects unused interfaces in the package.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	r := runner{}

	analyzer := &analysis.Analyzer{
		Name:     "unused",
		Doc:      "Detects interfaces which are not used anywhere in the same package where they are defined.",
		URL:      "https://pkg.go.dev/github.com/uudashr/iface/unused",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      r.run,
	}

	analyzer.Flags.BoolVar(&r.debug, "nerd", false, "enable nerd mode")
	analyzer.Flags.StringVar(&r.exclude, "exclude", "", "comma-separated list of packages to exclude from the check")

	return analyzer
}

type runner struct {
	debug   bool
	exclude string
}

func (r *runner) run(pass *analysis.Pass) (any, error) {
	excludes := strings.Split(r.exclude, ",")
	if slices.Contains(excludes, pass.Pkg.Path()) {
		return nil, nil
	}

	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Collect all interface type declarations
	ifaceDecls := make(map[string]*ast.TypeSpec)
	genDecls := make(map[string]*ast.GenDecl) // ifaceName -> GenDecl

	nodeFilter := []ast.Node{
		(*ast.GenDecl)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		decl, ok := n.(*ast.GenDecl)
		if !ok {
			return
		}

		if r.debug {
			fmt.Printf("GenDecl: %v specs=%d\n", decl.Tok, len(decl.Specs))
		}

		if decl.Tok != token.TYPE {
			return
		}

		for i, spec := range decl.Specs {
			if r.debug {
				fmt.Printf(" spec[%d]: %v %v\n", i, spec, reflect.TypeOf(spec))
			}

			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			_, ok = ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			if r.debug {
				fmt.Println(" Interface type declaration:", ts.Name.Name, ts.Pos())
			}

			dir := directive.ParseIgnore(decl.Doc)
			if dir != nil && dir.ShouldIgnore(pass.Analyzer.Name) {
				// skip due to ignore directive
				continue
			}

			ifaceDecls[ts.Name.Name] = ts
			genDecls[ts.Name.Name] = decl
		}
	})

	if r.debug {
		var ifaceNames []string
		for name := range ifaceDecls {
			ifaceNames = append(ifaceNames, name)
		}

		fmt.Println("Declared interfaces:", ifaceNames)
	}

	// Inspect whether the interface is used within the package
	nodeFilter = []ast.Node{
		(*ast.Ident)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return
		}

		ts, ok := ifaceDecls[ident.Name]
		if !ok {
			return
		}

		if ts.Pos() == ident.Pos() {
			// The identifier is the interface type declaration
			return
		}

		delete(ifaceDecls, ident.Name)
		delete(genDecls, ident.Name)
	})

	if r.debug {
		fmt.Printf("Package %s %s\n", pass.Pkg.Path(), pass.Pkg.Name())
	}

	for name, ts := range ifaceDecls {
		decl := genDecls[name]

		var node ast.Node
		if len(decl.Specs) == 1 {
			node = decl
		} else {
			node = ts
		}

		msg := fmt.Sprintf("interface '%s' is declared but not used within the package", name)
		pass.Report(analysis.Diagnostic{
			Pos:     ts.Pos(),
			Message: msg,
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: "Remove the unused interface declaration",
					TextEdits: []analysis.TextEdit{
						{
							Pos:     node.Pos(),
							End:     node.End(),
							NewText: []byte{},
						},
					},
				},
			},
		})
	}

	return nil, nil
}
