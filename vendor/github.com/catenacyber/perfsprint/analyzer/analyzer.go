// Package analyzer provides a static analysis tool that checks that fmt.Sprintf can be replaced with a faster alternative.
package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"golang.org/x/tools/go/analysis"
)

type optionInt struct {
	enabled bool
	intConv bool
}

type optionErr struct {
	enabled  bool
	errError bool
	errorf   bool
}

type optionStr struct {
	enabled   bool
	sprintf1  bool
	strconcat bool
}

type optionConcatLoop struct {
	enabled  bool
	otherOps bool
}

type perfSprint struct {
	intFormat  optionInt
	errFormat  optionErr
	strFormat  optionStr
	concatLoop optionConcatLoop

	boolFormat bool
	hexFormat  bool

	fiximports bool
}

func newPerfSprint() *perfSprint {
	return &perfSprint{
		intFormat:  optionInt{enabled: true, intConv: true},
		errFormat:  optionErr{enabled: true, errError: false, errorf: true},
		strFormat:  optionStr{enabled: true, sprintf1: true, strconcat: true},
		concatLoop: optionConcatLoop{enabled: true, otherOps: false},
		boolFormat: true,
		hexFormat:  true,
		fiximports: true,
	}
}

const (
	// checkerErrorFormat checks for error formatting.
	checkerErrorFormat = "error-format"
	// checkerIntegerFormat checks for integer formatting.
	checkerIntegerFormat = "integer-format"
	// checkerStringFormat checks for string formatting.
	checkerStringFormat = "string-format"
	// checkerBoolFormat checks for bool formatting.
	checkerBoolFormat = "bool-format"
	// checkerHexFormat checks for hexadecimal formatting.
	checkerHexFormat = "hex-format"
	// checkerConcatLoop checks for concatenation in loop.
	checkerConcatLoop = "concat-loop"
	// checkerFixImports fix needed imports from other fixes.
	checkerFixImports = "fiximports"
)

func New() *analysis.Analyzer {
	n := newPerfSprint()
	r := &analysis.Analyzer{
		Name:     "perfsprint",
		URL:      "https://github.com/catenacyber/perfsprint",
		Doc:      "Checks that fmt.Sprintf can be replaced with a faster alternative.",
		Run:      n.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	r.Flags.BoolVar(&n.intFormat.enabled, checkerIntegerFormat, n.intFormat.enabled, "enable/disable optimization of integer formatting")
	r.Flags.BoolVar(&n.intFormat.intConv, "int-conversion", n.intFormat.intConv, "optimizes even if it requires an int or uint type cast")
	r.Flags.BoolVar(&n.errFormat.enabled, checkerErrorFormat, n.errFormat.enabled, "enable/disable optimization of error formatting")
	r.Flags.BoolVar(&n.errFormat.errError, "err-error", n.errFormat.errError, "optimizes into err.Error() even if it is only equivalent for non-nil errors")
	r.Flags.BoolVar(&n.errFormat.errorf, "errorf", n.errFormat.errorf, "optimizes fmt.Errorf")

	r.Flags.BoolVar(&n.boolFormat, checkerBoolFormat, n.boolFormat, "enable/disable optimization of bool formatting")
	r.Flags.BoolVar(&n.hexFormat, checkerHexFormat, n.hexFormat, "enable/disable optimization of hex formatting")
	r.Flags.BoolVar(&n.concatLoop.enabled, checkerConcatLoop, n.concatLoop.enabled, "enable/disable optimization of concat loop")
	r.Flags.BoolVar(&n.concatLoop.otherOps, "loop-other-ops", n.concatLoop.otherOps, "optimization of concat loop even with other operations")
	r.Flags.BoolVar(&n.strFormat.enabled, checkerStringFormat, n.strFormat.enabled, "enable/disable optimization of string formatting")
	r.Flags.BoolVar(&n.strFormat.sprintf1, "sprintf1", n.strFormat.sprintf1, "optimizes fmt.Sprintf with only one argument")
	r.Flags.BoolVar(&n.strFormat.strconcat, "strconcat", n.strFormat.strconcat, "optimizes into strings concatenation")
	r.Flags.BoolVar(&n.fiximports, checkerFixImports, n.fiximports, "fix needed imports from other fixes")

	return r
}

// true if verb is a format string that could be replaced with concatenation.
func isConcatable(verb string) bool {
	hasPrefix := (strings.HasPrefix(verb, "%s") && !strings.Contains(verb, "%[1]s")) ||
		(strings.HasPrefix(verb, "%[1]s") && !strings.Contains(verb, "%s"))
	hasSuffix := (strings.HasSuffix(verb, "%s") && !strings.Contains(verb, "%[1]s")) ||
		(strings.HasSuffix(verb, "%[1]s") && !strings.Contains(verb, "%s"))

	if strings.Count(verb, "%[1]s") > 1 {
		return false
	}
	// TODO handle case hasPrefix and hasSuffix
	return (hasPrefix || hasSuffix) && !(hasPrefix && hasSuffix) //nolint:staticcheck
}

func isStringAdd(st *ast.AssignStmt, idname string) ast.Expr {
	// right is one
	if len(st.Rhs) == 1 {
		// right is addition
		add, ok := st.Rhs[0].(*ast.BinaryExpr)
		if ok && add.Op == token.ADD {
			// right is addition to same ident name
			x, ok := add.X.(*ast.Ident)
			if ok && x.Name == idname {
				return add.Y
			}
		}
	}
	return nil
}

func (n *perfSprint) reportConcatLoop(pass *analysis.Pass, neededPackages map[string]map[string]struct{}, node ast.Node, adds map[string][]*ast.AssignStmt) *analysis.Diagnostic {
	fname := pass.Fset.File(node.Pos()).Name()
	if _, ok := neededPackages[fname]; !ok {
		neededPackages[fname] = make(map[string]struct{})
	}
	// note that we will need strings package
	neededPackages[fname]["strings"] = struct{}{}

	// sort for reproducibility
	keys := make([]string, 0, len(adds))
	for k := range adds {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// use line number to define a unique variable name for the strings Builder
	loopStartLine := pass.Fset.Position(node.Pos()).Line

	// If the loop does more with the string than concatenations
	// add a TODO/FIXME comment that the fix is likely incomplete/incorrect
	addTODO := ""
	ast.Inspect(node, func(n ast.Node) bool {
		if len(addTODO) > 0 {
			// already found one, stop recursing
			return false
		}
		switch x := n.(type) {
		case *ast.AssignStmt:
			// skip if this is one string concatenation that we are fixing
			if (x.Tok == token.ASSIGN || x.Tok == token.ADD_ASSIGN) && len(x.Lhs) == 1 {
				id, ok := x.Lhs[0].(*ast.Ident)
				if ok {
					_, ok = adds[id.Name]
					if ok {
						if x.Tok == token.ASSIGN && isStringAdd(x, id.Name) == nil {
							addTODO = id.Name
						}
						return false
					}
				}
			}
		case *ast.Ident:
			_, ok := adds[x.Name]
			if ok {
				// The variable name is used in some place else
				addTODO = x.Name
				return false
			}
		}
		return true
	})

	prefix := ""
	suffix := ""
	if len(addTODO) > 0 {
		if !n.concatLoop.otherOps {
			return nil
		}
		prefix = fmt.Sprintf("// FIXME check usages of string identifier %s (and mayber others) in loop\n", addTODO)
	}
	// The fix contains 3 parts
	// before the loop: declare the strings Builders
	// during the loop: replace concatenation with Builder.WriteString
	// after the loop: use the Builder.String to append to the pre-existing string
	var prefixSb203 strings.Builder
	var suffixSb203 strings.Builder
	for _, k := range keys {
		// lol
		prefixSb203.WriteString(fmt.Sprintf("var %sSb%d strings.Builder\n", k, loopStartLine))
		suffixSb203.WriteString(fmt.Sprintf("\n%s += %sSb%d.String()", k, k, loopStartLine))
	}
	prefix += prefixSb203.String()
	suffix += suffixSb203.String()
	te := []analysis.TextEdit{
		{
			Pos:     node.Pos(),
			End:     node.Pos(),
			NewText: []byte(prefix),
		},
	}
	for _, k := range keys {
		v := adds[k]
		for _, st := range v {
			// s += "x" -> use "x"
			added := st.Rhs[0]
			if st.Tok == token.ASSIGN {
				// s = s + "x" -> use just "x", not `s + "x"`
				added = isStringAdd(st, k)
			}
			te = append(te, analysis.TextEdit{
				Pos:     st.Pos(),
				End:     added.Pos(),
				NewText: []byte(fmt.Sprintf("%sSb%d.WriteString(", k, loopStartLine)),
			})
			te = append(te, analysis.TextEdit{
				Pos:     added.End(),
				End:     added.End(),
				NewText: []byte(")"),
			})
		}
	}
	te = append(te, analysis.TextEdit{
		Pos:     node.End(),
		End:     node.End(),
		NewText: []byte(suffix),
	})

	return newAnalysisDiagnostic(
		checkerConcatLoop,
		adds[keys[0]][0],
		"string concatenation in a loop",
		[]analysis.SuggestedFix{
			{
				Message:   "Use a strings.Builder",
				TextEdits: te,
			},
		},
	)
}

func (n *perfSprint) runConcatLoop(pass *analysis.Pass, neededPackages map[string]map[string]struct{}) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	// 2 different kinds of loops in go
	nodeFilter := []ast.Node{
		(*ast.RangeStmt)(nil),
		(*ast.ForStmt)(nil),
	}
	insp.Preorder(nodeFilter, func(node ast.Node) {
		// set of variable names declared insied the loop
		declInLoop := make(map[string]bool)
		var bl []ast.Stmt
		// just take the list of instruction of the loop
		switch ra := node.(type) {
		case *ast.RangeStmt:
			bl = ra.Body.List
		case *ast.ForStmt:
			bl = ra.Body.List
		}
		// set of results : mapping a variable name to a list of statements like `s +=`
		// one loop may be bad for multiple string variables,
		// each being concatenated in multiple statements
		adds := make(map[string][]*ast.AssignStmt)
		for bs := 0; bs < len(bl); bs++ {
			switch st := bl[bs].(type) {
			case *ast.IfStmt:
				// explore breadth first, but go inside the if/else blocks
				if st.Body != nil {
					bl = append(bl, st.Body.List...)
				}
				el, ok := st.Else.(*ast.BlockStmt)
				if ok && el != nil {
					bl = append(bl, el.List...)
				}
			case *ast.DeclStmt:
				// identifiers defined within loop do not count
				de, ok := st.Decl.(*ast.GenDecl)
				if !ok {
					break
				}
				if len(de.Specs) != 1 {
					break
				}
				// is it possible to have len(de.Specs) > 1 for ValueSpec ?
				vs, ok := de.Specs[0].(*ast.ValueSpec)
				if !ok {
					break
				}
				for n := range vs.Names {
					declInLoop[vs.Names[n].Name] = true
				}
			case *ast.AssignStmt:
				for n := range st.Lhs {
					id, ok := st.Lhs[n].(*ast.Ident)
					if !ok {
						break
					}
					switch st.Tok {
					case token.DEFINE:
						declInLoop[id.Name] = true
					case token.ASSIGN, token.ADD_ASSIGN:
						if n > 0 {
							// do not search bugs for multi-assign
							break
						}
						_, local := declInLoop[id.Name]
						if local {
							break
						}
						ti, ok := pass.TypesInfo.Types[id]
						if !ok || ti.Type.String() != "string" {
							break
						}
						if st.Tok == token.ASSIGN {
							if isStringAdd(st, id.Name) == nil {
								break
							}
						}
						// found a bad string concat in the loop
						adds[id.Name] = append(adds[id.Name], st)
					}
				}
			}
		}
		if len(adds) > 0 {
			d := n.reportConcatLoop(pass, neededPackages, node, adds)
			if d != nil {
				pass.Report(*d)
			}
		}
	})
}

func (n *perfSprint) fixImports(pass *analysis.Pass, neededPackages map[string]map[string]struct{}, removedFmtUsages map[string]int) {
	if !n.fiximports {
		return
	}
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() == "fmt" {
			insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
			nodeFilter := []ast.Node{
				(*ast.SelectorExpr)(nil),
			}
			insp.Preorder(nodeFilter, func(node ast.Node) {
				selec := node.(*ast.SelectorExpr)
				selecok, ok := selec.X.(*ast.Ident)
				if ok {
					pkgname, ok := pass.TypesInfo.ObjectOf(selecok).(*types.PkgName)
					if ok && pkgname.Name() == pkg.Name() {
						fname := pass.Fset.File(pkgname.Pos()).Name()
						removedFmtUsages[fname]--
					}
				}
			})
		} else if pkg.Path() == "errors" || pkg.Path() == "strconv" || pkg.Path() == "encoding/hex" || pkg.Path() == "strings" {
			insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
			nodeFilter := []ast.Node{
				(*ast.ImportSpec)(nil),
			}
			insp.Preorder(nodeFilter, func(node ast.Node) {
				gd := node.(*ast.ImportSpec)
				if gd.Path.Value == strconv.Quote(pkg.Path()) {
					fname := pass.Fset.File(gd.Pos()).Name()
					if _, ok := neededPackages[fname]; ok {
						delete(neededPackages[fname], pkg.Path())
					}
				}
			})
		}
	}
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.File)(nil),
	}
	insp.Preorder(nodeFilter, func(node ast.Node) {
		gd := node.(*ast.File)
		fname := pass.Fset.File(gd.Pos()).Name()
		removed, hasFmt := removedFmtUsages[fname]
		if (!hasFmt || removed < 0) && len(neededPackages[fname]) == 0 {
			return
		}
		fix := ""
		var ar analysis.Range
		ar = gd.Decls[0]
		start := gd.Decls[0].Pos()
		end := gd.Decls[0].Pos()
		if len(gd.Imports) == 0 {
			fix += "import (\n"
		} else {
			id := gd.Decls[0].(*ast.GenDecl)
			start = id.Specs[0].Pos()
			end = id.Specs[0].Pos()
			if removedFmtUsages[fname] >= 0 {
				for sp := range id.Specs {
					is := id.Specs[sp].(*ast.ImportSpec)
					if is.Path.Value == strconv.Quote("fmt") {
						ar = is
						start = is.Pos()
						end = is.End()
						break
					}
				}
			}
		}
		keys := make([]string, 0, len(neededPackages[fname]))
		for k := range neededPackages[fname] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			already := false
			knames := strings.Split(k, "/")
			kname := knames[len(knames)-1]
			for i := range gd.Imports { // quadratic
				if (gd.Imports[i].Name != nil && gd.Imports[i].Name.Name == kname) || (gd.Imports[i].Name == nil && strings.HasSuffix(gd.Imports[i].Path.Value, "/"+kname+`"`)) {
					already = true
				}
			}
			if already {
				fix = fix + "\t\"" + k + "\" //TODO FIXME\n"
			} else {
				fix = fix + "\t\"" + k + "\"\n"
			}
		}
		if len(gd.Imports) == 0 {
			fix += ")\n"
		}
		pass.Report(*newAnalysisDiagnostic(
			checkerFixImports,
			ar,
			"Fix imports",
			[]analysis.SuggestedFix{
				{
					Message: "Fix imports",
					TextEdits: []analysis.TextEdit{{
						Pos:     start,
						End:     end,
						NewText: []byte(fix),
					}},
				},
			}))
	})
}

func (n *perfSprint) run(pass *analysis.Pass) (interface{}, error) {
	if !n.intFormat.enabled {
		n.intFormat.intConv = false
	}
	if !n.errFormat.enabled {
		n.errFormat.errError = false
		n.errFormat.errorf = false
	}
	if !n.strFormat.enabled {
		n.strFormat.sprintf1 = false
		n.strFormat.strconcat = false
	}

	neededPackages := make(map[string]map[string]struct{})
	if n.concatLoop.enabled {
		n.runConcatLoop(pass, neededPackages)
	}
	removedFmtUsages := make(map[string]int)
	var fmtSprintObj, fmtSprintfObj, fmtErrorfObj types.Object
	for _, pkg := range pass.Pkg.Imports() {
		if pkg.Path() == "fmt" {
			fmtSprintObj = pkg.Scope().Lookup("Sprint")
			fmtSprintfObj = pkg.Scope().Lookup("Sprintf")
			fmtErrorfObj = pkg.Scope().Lookup("Errorf")
		}
	}
	if fmtSprintfObj == nil && fmtSprintObj == nil && fmtErrorfObj == nil {
		if len(neededPackages) > 0 {
			n.fixImports(pass, neededPackages, removedFmtUsages)
		}
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}
	insp.Preorder(nodeFilter, func(node ast.Node) {
		call := node.(*ast.CallExpr)
		called, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		calledObj := pass.TypesInfo.ObjectOf(called.Sel)

		var (
			fn    string
			verb  string
			value ast.Expr
			err   error
		)
		switch {
		case calledObj == fmtErrorfObj && len(call.Args) == 1 && n.errFormat.errorf:
			fn = "fmt.Errorf"
			verb = "%s"
			value = call.Args[0]

		case calledObj == fmtSprintObj && len(call.Args) == 1:
			fn = "fmt.Sprint"
			verb = "%v"
			value = call.Args[0]

		case calledObj == fmtSprintfObj && len(call.Args) == 1 && n.strFormat.sprintf1:
			fn = "fmt.Sprintf"
			verb = "%s"
			value = call.Args[0]

		case calledObj == fmtSprintfObj && len(call.Args) == 2:
			verbLit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return
			}
			verb, err = strconv.Unquote(verbLit.Value)
			if err != nil {
				// Probably unreachable.
				return
			}
			// one single explicit arg is simplified
			if strings.HasPrefix(verb, "%[1]") {
				verb = "%" + verb[4:]
			}

			fn = "fmt.Sprintf"
			value = call.Args[1]

		default:
			return
		}

		switch verb {
		default:
			if fn == "fmt.Sprintf" && isConcatable(verb) && n.strFormat.strconcat {
				break
			}
			return
		case "%d", "%v", "%x", "%t", "%s":
		}

		valueType := pass.TypesInfo.TypeOf(value)
		a, isArray := valueType.(*types.Array)
		s, isSlice := valueType.(*types.Slice)

		var d *analysis.Diagnostic
		switch {
		case isBasicType(valueType, types.String) && oneOf(verb, "%v", "%s"):
			fname := pass.Fset.File(call.Pos()).Name()
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			removedFmtUsages[fname]++
			if fn == "fmt.Errorf" && n.errFormat.enabled {
				neededPackages[fname]["errors"] = struct{}{}
				d = newAnalysisDiagnostic(
					checkerErrorFormat,
					call,
					fn+" can be replaced with errors.New",
					[]analysis.SuggestedFix{
						{
							Message: "Use errors.New",
							TextEdits: []analysis.TextEdit{{
								Pos:     call.Pos(),
								End:     value.Pos(),
								NewText: []byte("errors.New("),
							}},
						},
					},
				)
			} else if fn != "fmt.Errorf" && n.strFormat.enabled {
				d = newAnalysisDiagnostic(
					checkerStringFormat,
					call,
					fn+" can be replaced with just using the string",
					[]analysis.SuggestedFix{
						{
							Message: "Just use string value",
							TextEdits: []analysis.TextEdit{{
								Pos:     call.Pos(),
								End:     call.End(),
								NewText: []byte(formatNode(pass.Fset, value)),
							}},
						},
					},
				)
			}
		case types.Implements(valueType, errIface) && oneOf(verb, "%v", "%s") && n.errFormat.errError:
			// known false positive if this error is nil
			// fmt.Sprint(nil) does not panic like nil.Error() does
			errMethodCall := formatNode(pass.Fset, value) + ".Error()"
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			d = newAnalysisDiagnostic(
				checkerErrorFormat,
				call,
				fn+" can be replaced with "+errMethodCall,
				[]analysis.SuggestedFix{
					{
						Message: "Use " + errMethodCall,
						TextEdits: []analysis.TextEdit{{
							Pos:     call.Pos(),
							End:     call.End(),
							NewText: []byte(errMethodCall),
						}},
					},
				},
			)

		case isBasicType(valueType, types.Bool) && oneOf(verb, "%v", "%t") && n.boolFormat:
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["strconv"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerBoolFormat,
				call,
				fn+" can be replaced with faster strconv.FormatBool",
				[]analysis.SuggestedFix{
					{
						Message: "Use strconv.FormatBool",
						TextEdits: []analysis.TextEdit{{
							Pos:     call.Pos(),
							End:     value.Pos(),
							NewText: []byte("strconv.FormatBool("),
						}},
					},
				},
			)

		case isArray && isBasicType(a.Elem(), types.Uint8) && oneOf(verb, "%x") && n.hexFormat:
			if _, ok := value.(*ast.Ident); !ok {
				// Doesn't support array literals.
				return
			}

			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["encoding/hex"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerHexFormat,
				call,
				fn+" can be replaced with faster hex.EncodeToString",
				[]analysis.SuggestedFix{
					{
						Message: "Use hex.EncodeToString",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     call.Pos(),
								End:     value.Pos(),
								NewText: []byte("hex.EncodeToString("),
							},
							{
								Pos:     value.End(),
								End:     value.End(),
								NewText: []byte("[:]"),
							},
						},
					},
				},
			)
		case isSlice && isBasicType(s.Elem(), types.Uint8) && oneOf(verb, "%x") && n.hexFormat:
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["encoding/hex"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerHexFormat,
				call,
				fn+" can be replaced with faster hex.EncodeToString",
				[]analysis.SuggestedFix{
					{
						Message: "Use hex.EncodeToString",
						TextEdits: []analysis.TextEdit{{
							Pos:     call.Pos(),
							End:     value.Pos(),
							NewText: []byte("hex.EncodeToString("),
						}},
					},
				},
			)

		case isBasicType(valueType, types.Int8, types.Int16, types.Int32) && oneOf(verb, "%v", "%d") && n.intFormat.intConv:
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["strconv"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerIntegerFormat,
				call,
				fn+" can be replaced with faster strconv.Itoa",
				[]analysis.SuggestedFix{
					{
						Message: "Use strconv.Itoa",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     call.Pos(),
								End:     value.Pos(),
								NewText: []byte("strconv.Itoa(int("),
							},
							{
								Pos:     value.End(),
								End:     value.End(),
								NewText: []byte(")"),
							},
						},
					},
				},
			)
		case isBasicType(valueType, types.Int) && oneOf(verb, "%v", "%d") && n.intFormat.enabled:
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["strconv"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerIntegerFormat,
				call,
				fn+" can be replaced with faster strconv.Itoa",
				[]analysis.SuggestedFix{
					{
						Message: "Use strconv.Itoa",
						TextEdits: []analysis.TextEdit{{
							Pos:     call.Pos(),
							End:     value.Pos(),
							NewText: []byte("strconv.Itoa("),
						}},
					},
				},
			)
		case isBasicType(valueType, types.Int64) && oneOf(verb, "%v", "%d") && n.intFormat.enabled:
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["strconv"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerIntegerFormat,
				call,
				fn+" can be replaced with faster strconv.FormatInt",
				[]analysis.SuggestedFix{
					{
						Message: "Use strconv.FormatInt",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     call.Pos(),
								End:     value.Pos(),
								NewText: []byte("strconv.FormatInt("),
							},
							{
								Pos:     value.End(),
								End:     value.End(),
								NewText: []byte(", 10"),
							},
						},
					},
				},
			)

		case isBasicType(valueType, types.Uint8, types.Uint16, types.Uint32, types.Uint) && oneOf(verb, "%v", "%d", "%x") && n.intFormat.intConv:
			base := []byte("), 10")
			if verb == "%x" {
				base = []byte("), 16")
			}
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["strconv"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerIntegerFormat,
				call,
				fn+" can be replaced with faster strconv.FormatUint",
				[]analysis.SuggestedFix{
					{
						Message: "Use strconv.FormatUint",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     call.Pos(),
								End:     value.Pos(),
								NewText: []byte("strconv.FormatUint(uint64("),
							},
							{
								Pos:     value.End(),
								End:     value.End(),
								NewText: base,
							},
						},
					},
				},
			)
		case isBasicType(valueType, types.Uint64) && oneOf(verb, "%v", "%d", "%x") && n.intFormat.enabled:
			base := []byte(", 10")
			if verb == "%x" {
				base = []byte(", 16")
			}
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			if _, ok := neededPackages[fname]; !ok {
				neededPackages[fname] = make(map[string]struct{})
			}
			neededPackages[fname]["strconv"] = struct{}{}
			d = newAnalysisDiagnostic(
				checkerIntegerFormat,
				call,
				fn+" can be replaced with faster strconv.FormatUint",
				[]analysis.SuggestedFix{
					{
						Message: "Use strconv.FormatUint",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     call.Pos(),
								End:     value.Pos(),
								NewText: []byte("strconv.FormatUint("),
							},
							{
								Pos:     value.End(),
								End:     value.End(),
								NewText: base,
							},
						},
					},
				},
			)
		case isBasicType(valueType, types.String) && fn == "fmt.Sprintf" && isConcatable(verb) && n.strFormat.enabled:
			var fix string
			if strings.HasSuffix(verb, "%s") {
				fix = strings.ReplaceAll(strconv.Quote(verb[:len(verb)-2]), "%%", "%") + "+" + formatNode(pass.Fset, value)
			} else if strings.HasSuffix(verb, "%[1]s") {
				fix = strings.ReplaceAll(strconv.Quote(verb[:len(verb)-5]), "%%", "%") + "+" + formatNode(pass.Fset, value)
			} else if strings.HasPrefix(verb, "%s") {
				fix = formatNode(pass.Fset, value) + "+" + strings.ReplaceAll(strconv.Quote(verb[2:]), "%%", "%")
			} else {
				fix = formatNode(pass.Fset, value) + "+" + strings.ReplaceAll(strconv.Quote(verb[5:]), "%%", "%")
			}
			fname := pass.Fset.File(call.Pos()).Name()
			removedFmtUsages[fname]++
			d = newAnalysisDiagnostic(
				checkerStringFormat,
				call,
				fn+" can be replaced with string concatenation",
				[]analysis.SuggestedFix{
					{
						Message: "Use string concatenation",
						TextEdits: []analysis.TextEdit{{
							Pos:     call.Pos(),
							End:     call.End(),
							NewText: []byte(fix),
						}},
					},
				},
			)
		}

		if d != nil {
			pass.Report(*d)
		}
	})

	if len(removedFmtUsages) > 0 || len(neededPackages) > 0 {
		n.fixImports(pass, neededPackages, removedFmtUsages)
	}

	return nil, nil
}

var errIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func isBasicType(lhs types.Type, expected ...types.BasicKind) bool {
	for _, rhs := range expected {
		if types.Identical(lhs, types.Typ[rhs]) {
			return true
		}
	}
	return false
}

func formatNode(fset *token.FileSet, node ast.Node) string {
	buf := new(bytes.Buffer)
	if err := format.Node(buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

func oneOf[T comparable](v T, expected ...T) bool {
	for _, rhs := range expected {
		if v == rhs {
			return true
		}
	}
	return false
}
