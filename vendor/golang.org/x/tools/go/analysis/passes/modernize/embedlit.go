// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package modernize

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/edge"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/internal/analysis/analyzerutil"
	"golang.org/x/tools/internal/astutil"
	"golang.org/x/tools/internal/moreiters"
	"golang.org/x/tools/internal/versions"
)

var EmbedLitAnalyzer = &analysis.Analyzer{
	Name: "embedlit",
	Doc:  analyzerutil.MustExtractDoc(doc, "embedlit"),
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	Run: runEmbedLit,
	URL: "https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize#embedlit",
}

// TODO(mkalil): Handle other patterns such as:
// t := T{...}
// t.x = x
// ...
// =>
// t := T{..., x: x, ...}
func runEmbedLit(pass *analysis.Pass) (any, error) {
	var (
		inspect = pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
		info    = pass.TypesInfo
	)
	for curLit := range inspect.Root().Preorder((*ast.CompositeLit)(nil)) {
		var (
			edits       []analysis.TextEdit
			names       []string // names of the embedded field types that can be removed
			compLit     = curLit.Node().(*ast.CompositeLit)
			compLitType = info.TypeOf(compLit)
			check       func(*ast.CompositeLit)
		)
		check = func(lit *ast.CompositeLit) {
			for _, elt := range lit.Elts {
				// Can't promote an unkeyed field; would result in a syntax error.
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if innerLit := isEmbeddedFieldLit(info, compLitType, kv); innerLit != nil {
						// Emit edits to delete the unnecessary embedded field type specifier
						// and its closing brace.
						closingPos := innerLit.Rbrace
						if len(innerLit.Elts) > 0 {
							// Delete any inner trailing commas or white space. Extra trailing commas
							// would result in invalid code.
							closingPos = innerLit.Elts[len(innerLit.Elts)-1].End()
						}
						file := astutil.EnclosingFile(curLit)
						// Enable modernizer only for Go1.27.
						if !analyzerutil.FileUsesGoVersion(pass, file, versions.Go1_27) {
							return
						}
						// If any comments overlap with the range to delete, don't suggest a fix.
						if !moreiters.Empty(astutil.Comments(file, closingPos, innerLit.Rbrace+1)) {
							continue
						}
						edits = append(edits, []analysis.TextEdit{
							// T{U: U{f: v, ...}}
							//   -----         -
							{
								// Delete the key and the opening brace of the inner struct literal.
								Pos: kv.Pos(),
								End: innerLit.Lbrace + 1,
							},
							{
								// Delete the corresponding closing brace, including preceding
								// white space or commas. Failing to delete trailing commas may
								// result in invalid code.
								Pos: closingPos,
								End: innerLit.Rbrace + 1,
							},
						}...)
						names = append(names, kv.Key.(*ast.Ident).Name)
						check(innerLit)
					}
				}
			}
		}

		if curLit.ParentEdgeKind() != edge.KeyValueExpr_Value {
			compLit := curLit.Node().(*ast.CompositeLit)
			check(compLit) // non-nested comp lit
		}
		if len(edits) > 0 {
			pass.Report(analysis.Diagnostic{
				Pos:     curLit.Node().Pos(),
				End:     curLit.Node().End(),
				Message: "embedded field type can be removed from struct literal",
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message:   fmt.Sprintf("Remove embedded field type%s %s", cond(len(names) == 1, "", "s"), strings.Join(names, ", ")),
						TextEdits: edits,
					},
				},
			})
		}
	}
	return nil, nil
}

// isEmbeddedFieldLit determines whether elt is a KeyValueExpr "T: T{...}" for
// an embedded field for which we can safely remove its type.
// If so, it returns the corresponding CompositeLit.
// If elt contains an unkeyed field or ambiguous type, it returns nil.
func isEmbeddedFieldLit(info *types.Info, topLevelType types.Type, kv *ast.KeyValueExpr) *ast.CompositeLit {
	obj := keyedField(info, kv)
	if obj == nil || !obj.Embedded() {
		return nil
	}
	lit, ok := kv.Value.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	// We cannot remove this type if any of its nested composite elements have
	// unkeyed fields or are ambiguous, so we check for those conditions before
	// returning.
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil
		}
		obj := keyedField(info, kv)
		if obj == nil {
			return nil
		}
		k := kv.Key.(*ast.Ident) // can't fail
		// Cannot promote an ambiguous type, for example:
		// type T struct { A; B }
		// type A struct { x int }
		// type B struct { x int }
		// _ = T{A: A{x: 1}}
		// cannot be simplified to T{x: 1} because T has two different embedded fields called "x".
		parentObj, _, _ := types.LookupFieldOrMethod(topLevelType, true, obj.Pkg(), k.Name)
		if parentObj != obj {
			return nil
		}
	}
	return lit
}

// keyedField reports whether the key of kv is an embedded field type. If so, it
// returns the type of the embedded field, otherwise it returns nil.
func keyedField(info *types.Info, kv *ast.KeyValueExpr) *types.Var {
	k, ok := kv.Key.(*ast.Ident)
	if !ok {
		return nil
	}
	obj, ok := info.ObjectOf(k).(*types.Var)
	if !ok || !obj.IsField() {
		return nil
	}
	return obj
}
