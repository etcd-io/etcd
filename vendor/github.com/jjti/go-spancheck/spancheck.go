package spancheck

import (
	"go/ast"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/cfg"
)

const stackLen = 32

// spanType differentiates span types.
type spanType int

const (
	spanUnset         spanType = iota // not a span
	spanOpenTelemetry                 // from go.opentelemetry.io/otel
	spanOpenCensus                    // from go.opencensus.io/trace
)

const (
	selNameEnd         = "End"
	selNameSetStatus   = "SetStatus"
	selNameRecordError = "RecordError"
)

// SpanTypes is a list of all span types by name.
var SpanTypes = map[string]spanType{
	"opentelemetry": spanOpenTelemetry,
	"opencensus":    spanOpenCensus,
}

// this approach stolen from errcheck
// https://github.com/kisielk/errcheck/blob/7f94c385d0116ccc421fbb4709e4a484d98325ee/errcheck/errcheck.go#L22
var errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

// NewAnalyzerWithConfig returns a new analyzer configured with the Config passed in.
// Its config can be set for testing.
func NewAnalyzerWithConfig(config *Config) *analysis.Analyzer {
	return newAnalyzer(config)
}

func newAnalyzer(config *Config) *analysis.Analyzer {
	config.finalize()

	return &analysis.Analyzer{
		Name:  "spancheck",
		Doc:   "Checks for mistakes with OpenTelemetry/Census spans.",
		Flags: config.fs,
		Run:   run(config),
		Requires: []*analysis.Analyzer{
			ctrlflow.Analyzer,
			inspect.Analyzer,
		},
	}
}

func run(config *Config) func(*analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

		nodeFilter := []ast.Node{
			(*ast.FuncLit)(nil),  // f := func() {}
			(*ast.FuncDecl)(nil), // func foo() {}
		}
		inspect.Preorder(nodeFilter, func(n ast.Node) {
			runFunc(pass, n, config)
		})

		return nil, nil
	}
}

type spanVar struct {
	stmt     ast.Node
	id       *ast.Ident
	vr       *types.Var
	spanType spanType
}

// runFunc checks if the node is a function, has a span, and the span never has SetStatus set.
func runFunc(pass *analysis.Pass, node ast.Node, config *Config) {
	// copying https://cs.opensource.google/go/x/tools/+/master:go/analysis/passes/lostcancel/lostcancel.go

	// Find scope of function node
	var funcScope *types.Scope
	switch v := node.(type) {
	case *ast.FuncLit:
		funcScope = pass.TypesInfo.Scopes[v.Type]
	case *ast.FuncDecl:
		funcScope = pass.TypesInfo.Scopes[v.Type]
		fnSig := pass.TypesInfo.ObjectOf(v.Name).String()

		// Skip checking spans in this function if it's a custom starter/creator.
		if config.startSpanMatchersCustomRegex != nil && config.startSpanMatchersCustomRegex.MatchString(fnSig) {
			return
		}
	}

	// Maps each span variable to its defining ValueSpec/AssignStmt.
	spanVars := make(map[*ast.Ident]spanVar)

	// Find the set of span vars to analyze.
	stack := make([]ast.Node, 0, stackLen)
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncLit:
			if len(stack) > 0 {
				return false // don't stray into nested functions
			}
		case nil:
			stack = stack[:len(stack)-1] // pop
			return true
		}
		stack = append(stack, n) // push

		// Look for [{AssignStmt,ValueSpec} CallExpr SelectorExpr]:
		//
		//   ctx, span     := otel.Tracer("app").Start(...)
		//   ctx, span     = otel.Tracer("app").Start(...)
		//   var ctx, span = otel.Tracer("app").Start(...)
		sType, isStart := isSpanStart(pass.TypesInfo, n, config.startSpanMatchers)
		if !isStart {
			return true
		}

		if !isCall(stack[len(stack)-2]) {
			return true
		}

		stmt := stack[len(stack)-3]
		id := getID(stmt)
		if id == nil {
			pass.ReportRangef(n, "span is unassigned, probable memory leak")
			return true
		}

		if id.Name == "_" {
			pass.ReportRangef(id, "span is unassigned, probable memory leak")
		} else if v, ok := pass.TypesInfo.Uses[id].(*types.Var); ok {
			// If the span variable is defined outside function scope,
			// do not analyze it.
			if funcScope.Contains(v.Pos()) {
				spanVars[id] = spanVar{
					vr:       v,
					stmt:     stmt,
					id:       id,
					spanType: sType,
				}
			}
		} else if v, ok := pass.TypesInfo.Defs[id].(*types.Var); ok {
			spanVars[id] = spanVar{
				vr:       v,
				stmt:     stmt,
				id:       id,
				spanType: sType,
			}
		}

		return true
	})

	if len(spanVars) == 0 {
		return // no need to inspect CFG
	}

	// Obtain the CFG.
	cfgs := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	var g *cfg.CFG
	var sig *types.Signature
	switch node := node.(type) {
	case *ast.FuncDecl:
		sig, _ = pass.TypesInfo.Defs[node.Name].Type().(*types.Signature)
		g = cfgs.FuncDecl(node)
	case *ast.FuncLit:
		sig, _ = pass.TypesInfo.Types[node.Type].Type.(*types.Signature)
		g = cfgs.FuncLit(node)
	}
	if sig == nil {
		return // missing type information
	}

	// Check for missing calls.
	for _, sv := range spanVars {
		if config.endCheckEnabled {
			// Check if there's no End to the span.
			if ret := getMissingSpanCalls(pass, g, sv, selNameEnd, func(_ *analysis.Pass, ret *ast.ReturnStmt) *ast.ReturnStmt { return ret }, nil, config.startSpanMatchers); ret != nil {
				pass.ReportRangef(sv.stmt, "%s.End is not called on all paths, possible memory leak", sv.vr.Name())
				pass.ReportRangef(ret, "return can be reached without calling %s.End", sv.vr.Name())
			}
		}

		if config.setStatusEnabled {
			// Check if there's no SetStatus to the span setting an error.
			if ret := getMissingSpanCalls(pass, g, sv, selNameSetStatus, getErrorReturn, config.ignoreChecksSignatures, config.startSpanMatchers); ret != nil {
				pass.ReportRangef(sv.stmt, "%s.SetStatus is not called on all paths", sv.vr.Name())
				pass.ReportRangef(ret, "return can be reached without calling %s.SetStatus", sv.vr.Name())
			}
		}

		if config.recordErrorEnabled && sv.spanType == spanOpenTelemetry { // RecordError only exists in OpenTelemetry
			// Check if there's no RecordError to the span setting an error.
			if ret := getMissingSpanCalls(pass, g, sv, selNameRecordError, getErrorReturn, config.ignoreChecksSignatures, config.startSpanMatchers); ret != nil {
				pass.ReportRangef(sv.stmt, "%s.RecordError is not called on all paths", sv.vr.Name())
				pass.ReportRangef(ret, "return can be reached without calling %s.RecordError", sv.vr.Name())
			}
		}
	}
}

// isSpanStart reports whether n is tracer.Start()
func isSpanStart(info *types.Info, n ast.Node, startSpanMatchers []spanStartMatcher) (spanType, bool) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return spanUnset, false
	}

	fnSig := info.ObjectOf(sel.Sel).String()

	// Check if the function is a span start function.
	for _, matcher := range startSpanMatchers {
		if matcher.signature.MatchString(fnSig) {
			return matcher.spanType, true
		}
	}

	return 0, false
}

func isCall(n ast.Node) bool {
	_, ok := n.(*ast.CallExpr)
	return ok
}

func getID(node ast.Node) *ast.Ident {
	switch stmt := node.(type) {
	case *ast.ValueSpec:
		if len(stmt.Names) > 1 {
			return stmt.Names[1]
		} else if len(stmt.Names) == 1 {
			return stmt.Names[0]
		}
	case *ast.AssignStmt:
		if len(stmt.Lhs) > 1 {
			id, _ := stmt.Lhs[1].(*ast.Ident)
			return id
		} else if len(stmt.Lhs) == 1 {
			id, _ := stmt.Lhs[0].(*ast.Ident)
			return id
		}
	}
	return nil
}

// getMissingSpanCalls finds a path through the CFG, from stmt (which defines
// the 'span' variable v) to a return statement, that doesn't call the passed selector on the span.
func getMissingSpanCalls(
	pass *analysis.Pass,
	g *cfg.CFG,
	sv spanVar,
	selName string,
	checkErr func(pass *analysis.Pass, ret *ast.ReturnStmt) *ast.ReturnStmt,
	ignoreCheckSig *regexp.Regexp,
	spanStartMatchers []spanStartMatcher,
) *ast.ReturnStmt {
	// blockUses computes "uses" for each block, caching the result.
	memo := make(map[*cfg.Block]bool)
	blockUses := func(pass *analysis.Pass, b *cfg.Block) bool {
		res, ok := memo[b]
		if !ok {
			res = usesCall(pass, b.Nodes, sv, selName, ignoreCheckSig, spanStartMatchers, 0)
			memo[b] = res
		}
		return res
	}

	// Find the var's defining block in the CFG,
	// plus the rest of the statements of that block.
	var defBlock *cfg.Block
	var rest []ast.Node
outer:
	for _, b := range g.Blocks {
		for i, n := range b.Nodes {
			if n == sv.stmt {
				defBlock = b
				rest = b.Nodes[i+1:]
				break outer
			}
		}
	}

	// Is the call "used" in the remainder of its defining block?
	if usesCall(pass, rest, sv, selName, ignoreCheckSig, spanStartMatchers, 0) {
		return nil
	}

	// Does the defining block return without making the call?
	if ret := defBlock.Return(); ret != nil {
		return checkErr(pass, ret)
	}

	// Search the CFG depth-first for a path, from defblock to a
	// return block, in which v is never "used".
	seen := make(map[*cfg.Block]bool)
	var search func(blocks []*cfg.Block) *ast.ReturnStmt
	search = func(blocks []*cfg.Block) *ast.ReturnStmt {
		for _, b := range blocks {
			if seen[b] {
				continue
			}
			seen[b] = true

			// Skip successors that are not nested within this current block.
			if _, ok := nestedBlockTypes[b.Kind]; !ok {
				continue
			}

			// Prune the search if the block uses v.
			if blockUses(pass, b) {
				continue
			}

			// Found path to return statement?
			if ret := getErrorReturn(pass, b.Return()); ret != nil {
				return ret // found
			}

			// Recur
			if ret := getErrorReturn(pass, search(b.Succs)); ret != nil {
				return ret
			}
		}
		return nil
	}

	return search(defBlock.Succs)
}

var nestedBlockTypes = map[cfg.BlockKind]struct{}{
	cfg.KindBody:            {},
	cfg.KindForBody:         {},
	cfg.KindForLoop:         {},
	cfg.KindIfElse:          {},
	cfg.KindIfThen:          {},
	cfg.KindLabel:           {},
	cfg.KindRangeBody:       {},
	cfg.KindRangeLoop:       {},
	cfg.KindSelectCaseBody:  {},
	cfg.KindSelectAfterCase: {},
	cfg.KindSwitchCaseBody:  {},
	cfg.KindSwitchNextCase:  {},
}

// usesCall reports whether stmts contain a use of the selName call on variable v.
func usesCall(
	pass *analysis.Pass,
	stmts []ast.Node,
	sv spanVar,
	selName string,
	ignoreCheckSig *regexp.Regexp,
	startSpanMatchers []spanStartMatcher,
	depth int,
) bool {
	if depth > 1 { // for perf reasons, do not dive too deep thru func literals, just two levels deep.
		return false
	}

	cfgs := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)

	found, reAssigned := false, false
	for _, subStmt := range stmts {
		stack := []ast.Node{}
		ast.Inspect(subStmt, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				if len(stack) > 0 {
					g := cfgs.FuncLit(n)
					if g != nil && len(g.Blocks) > 0 {
						return usesCall(pass, g.Blocks[0].Nodes, sv, selName, ignoreCheckSig, startSpanMatchers, depth+1)
					}

					return false
				}
			case *ast.CallExpr:
				if ident, ok := n.Fun.(*ast.Ident); ok {
					fnSig := pass.TypesInfo.ObjectOf(ident).String()
					if ignoreCheckSig != nil && ignoreCheckSig.MatchString(fnSig) {
						found = true
						return false
					}
				}
			case *ast.DeferStmt:
				if n.Call == nil {
					break
				}

				f, ok := n.Call.Fun.(*ast.FuncLit)
				if !ok {
					break
				}

				if g := cfgs.FuncLit(f); g != nil && len(g.Blocks) > 0 {
					if selName == selNameEnd {
						// Check if all returning blocks call end.
						for _, b := range g.Blocks {
							if b.Return() != nil && !usesCall(
								pass,
								b.Nodes,
								sv,
								selName,
								ignoreCheckSig,
								startSpanMatchers,
								depth+1,
							) {
								return false
							}
						}

						found = true
						return false
					}

					for _, b := range g.Blocks {
						if usesCall(
							pass,
							b.Nodes,
							sv,
							selName,
							ignoreCheckSig,
							startSpanMatchers,
							depth+1,
						) {
							found = true
							return false
						}
					}
				}
			case nil:
				if len(stack) > 0 {
					stack = stack[:len(stack)-1] // pop
					return true
				}
				return false
			}
			stack = append(stack, n) // push

			// Check whether the span was assigned over top of its old value.
			_, isStart := isSpanStart(pass.TypesInfo, n, startSpanMatchers)
			if isStart {
				if id := getID(stack[len(stack)-3]); id != nil && id.Obj.Decl == sv.id.Obj.Decl {
					reAssigned = true
					return false
				}
			}

			if n, ok := n.(*ast.SelectorExpr); ok {
				// Selector (End, SetStatus, RecordError) hit.
				if n.Sel.Name == selName {
					id, ok := n.X.(*ast.Ident)
					found = ok && id.Obj != nil && id.Obj.Decl == sv.id.Obj.Decl
				}

				// Check if an ignore signature matches.
				fnSig := pass.TypesInfo.ObjectOf(n.Sel).String()
				if ignoreCheckSig != nil && ignoreCheckSig.MatchString(fnSig) {
					found = true
				}
			}

			return !found
		})
	}

	return found && !reAssigned
}

func getErrorReturn(pass *analysis.Pass, ret *ast.ReturnStmt) *ast.ReturnStmt {
	if ret == nil {
		return nil
	}

	for _, r := range ret.Results {
		if isErrorType(pass.TypesInfo.TypeOf(r)) {
			return ret
		}

		if r, ok := r.(*ast.CallExpr); ok {
			for _, err := range errorsByArg(pass, r) {
				if err {
					return ret
				}
			}
		}
	}

	return nil
}

// errorsByArg returns a slice s such that
// len(s) == number of return types of call
// s[i] == true iff return type at position i from left is an error type
//
// copied from https://github.com/kisielk/errcheck/blob/master/errcheck/errcheck.go
func errorsByArg(pass *analysis.Pass, call *ast.CallExpr) []bool {
	switch t := pass.TypesInfo.Types[call].Type.(type) {
	case *types.Named:
		// Single return
		return []bool{isErrorType(t)}
	case *types.Pointer:
		// Single return via pointer
		return []bool{isErrorType(t)}
	case *types.Tuple:
		// Multiple returns
		s := make([]bool, t.Len())
		for i := 0; i < t.Len(); i++ {
			switch et := t.At(i).Type().(type) {
			case *types.Named:
				// Single return
				s[i] = isErrorType(et)
			case *types.Pointer:
				// Single return via pointer
				s[i] = isErrorType(et)
			default:
				s[i] = false
			}
		}
		return s
	}
	return []bool{false}
}

func isErrorType(t types.Type) bool {
	return types.Implements(t, errorType)
}
