package rule

import (
	"fmt"
	"go/ast"
	"strconv"

	"github.com/mgechev/revive/lint"
)

// DotImportsRule forbids dot imports.
type DotImportsRule struct {
	allowedPackages allowPackages
}

// Apply applies the rule to given file.
func (r *DotImportsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	fileAst := file.AST
	walker := lintImports{
		file:    file,
		fileAst: fileAst,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
		allowPackages: r.allowedPackages,
	}

	ast.Walk(walker, fileAst)

	return failures
}

// Name returns the rule name.
func (*DotImportsRule) Name() string {
	return "dot-imports"
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *DotImportsRule) Configure(arguments lint.Arguments) error {
	r.allowedPackages = allowPackages{}
	if len(arguments) == 0 {
		return nil
	}

	args, ok := arguments[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid argument to the dot-imports rule. Expecting a k,v map, got %T", arguments[0])
	}

	for k, v := range args {
		if !isRuleOption(k, "allowedPackages") {
			continue
		}
		pkgs, ok := v.([]any)
		if !ok {
			return fmt.Errorf("invalid argument to the dot-imports rule, []string expected. Got '%v' (%T)", v, v)
		}
		for _, p := range pkgs {
			pkg, ok := p.(string)
			if !ok {
				return fmt.Errorf("invalid argument to the dot-imports rule, string expected. Got '%v' (%T)", p, p)
			}
			r.allowedPackages.add(pkg)
		}
	}
	return nil
}

type lintImports struct {
	file          *lint.File
	fileAst       *ast.File
	onFailure     func(lint.Failure)
	allowPackages allowPackages
}

func (w lintImports) Visit(_ ast.Node) ast.Visitor {
	for _, importSpec := range w.fileAst.Imports {
		isDotImport := importSpec.Name != nil && importSpec.Name.Name == "."
		if isDotImport && !w.allowPackages.isAllowedPackage(importSpec.Path.Value) {
			w.onFailure(lint.Failure{
				Confidence: 1,
				Failure:    "should not use dot imports",
				Node:       importSpec,
				Category:   lint.FailureCategoryImports,
			})
		}
	}
	return nil
}

type allowPackages map[string]struct{}

func (ap allowPackages) add(pkg string) {
	ap[strconv.Quote(pkg)] = struct{}{} // import path strings are with double quotes
}

func (ap allowPackages) isAllowedPackage(pkg string) bool {
	_, allowed := ap[pkg]
	return allowed
}
