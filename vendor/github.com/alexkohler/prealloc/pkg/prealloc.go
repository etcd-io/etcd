package pkg

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

type sliceDeclaration struct {
	name      string
	pos       token.Pos
	level     int      // Nesting level of this slice. Will be disqualified if appended at a deeper level.
	lenExpr   ast.Expr // Initial length of this slice.
	exclude   bool     // Whether this slice has been disqualified due to an unsupported pattern.
	hasReturn bool     // Whether a return statement has been found after the first append. Any subsequent appends will disqualify this slice in simple mode.
	assigning bool     // Whether this slice is currently being assigned the result of an append.
	detached  bool     // Whether this slice has been appended without reassignment. Will be disqualified if this happens more than once.
}

type sliceAppend struct {
	index     int      // Index of the target slice.
	countExpr ast.Expr // Number of items appended.
}

type returnsVisitor struct {
	// flags
	simple            bool
	includeRangeLoops bool
	includeForLoops   bool
	pass              *analysis.Pass
	// visitor fields
	sliceDeclarations []*sliceDeclaration
	sliceAppends      []*sliceAppend
	loopVars          []ast.Expr
	level             int  // Current nesting level. Loops do not increment the level.
	hasReturn         bool // Whether a return statement has been found. Slices appended before and after a return are disqualified in simple mode.
	hasGoto           bool // Whether a goto statement has been found. Goto disqualifies pending and subsequent slices in simple mode.
	hasBranch         bool // Whether a branch statement has been found. Loops with branch statements are unsupported in simple mode.
}

func Check(pass *analysis.Pass, simple, includeRangeLoops, includeForLoops bool) {
	retVis := &returnsVisitor{
		simple:            simple,
		includeRangeLoops: includeRangeLoops,
		includeForLoops:   includeForLoops,
		pass:              pass,
	}
	for _, f := range pass.Files {
		ast.Walk(retVis, f)
	}
}

func (v *returnsVisitor) Visit(node ast.Node) ast.Visitor {
	switch s := node.(type) {
	case *ast.FuncDecl:
		if s.Body == nil {
			return nil
		}
		v.level = 0
		v.hasReturn = false
		v.hasGoto = false
		ast.Walk(v, s.Body)
		return nil

	case *ast.FuncLit:
		if s.Body == nil {
			return nil
		}
		wasReturn := v.hasReturn
		wasGoto := v.hasGoto
		v.hasReturn = false
		ast.Walk(v, s.Body)
		v.hasReturn = wasReturn
		v.hasGoto = wasGoto
		return nil

	case *ast.BlockStmt:
		declIdx := len(v.sliceDeclarations)
		appendIdx := len(v.sliceAppends)
		v.level++
		for _, stmt := range s.List {
			ast.Walk(v, stmt)
		}
		v.level--

		buf := bytes.NewBuffer(nil)
		for i := declIdx; i < len(v.sliceDeclarations); i++ {
			sliceDecl := v.sliceDeclarations[i]
			if sliceDecl.exclude || v.hasGoto {
				continue
			}

			capExpr := sliceDecl.lenExpr
			for j := appendIdx; j < len(v.sliceAppends); j++ {
				if v.sliceAppends[j] != nil && v.sliceAppends[j].index == i {
					capExpr = addIntExpr(capExpr, v.sliceAppends[j].countExpr)
				}
			}
			if capExpr == sliceDecl.lenExpr {
				// nothing appended
				continue
			}
			if capVal, ok := intValue(capExpr); ok && capVal <= 0 {
				continue
			}

			buf.Reset()
			buf.WriteString("Consider preallocating ")
			buf.WriteString(sliceDecl.name)
			if capExpr != nil {
				undo := buf.Len()
				buf.WriteString(" with capacity ")
				if format.Node(buf, token.NewFileSet(), capExpr) != nil {
					buf.Truncate(undo)
				}
			}
			v.pass.Report(analysis.Diagnostic{
				Pos:     sliceDecl.pos,
				Message: buf.String(),
			})
		}

		// discard slices and associated appends that are falling out of scope
		v.sliceDeclarations = v.sliceDeclarations[:declIdx]
		for i := appendIdx; i < len(v.sliceAppends); i++ {
			if v.sliceAppends[i] != nil {
				if v.sliceAppends[i].index >= declIdx {
					v.sliceAppends[i] = nil
				} else {
					appendIdx = i + 1
				}
			}
		}
		v.sliceAppends = v.sliceAppends[:appendIdx]
		return nil

	case *ast.ValueSpec:
		var isArrayOrSlice bool
		if t := v.pass.TypesInfo.TypeOf(s.Type); t != nil {
			switch t.Underlying().(type) {
			case *types.Array, *types.Slice:
				isArrayOrSlice = true
			}
		}
		for i, name := range s.Names {
			var lenExpr ast.Expr
			if i >= len(s.Values) {
				if !isArrayOrSlice {
					continue
				}
				lenExpr = intExpr(0)
			} else if lenExpr = v.isCreateArray(s.Values[i]); lenExpr == nil {
				if id, ok := s.Values[i].(*ast.Ident); !ok || id.Name != "nil" {
					continue
				}
				lenExpr = intExpr(0)
			}
			v.sliceDeclarations = append(v.sliceDeclarations, &sliceDeclaration{name: name.Name, pos: s.Pos(), level: v.level, lenExpr: lenExpr})
		}

	case *ast.AssignStmt:
		if len(v.loopVars) > 0 {
			if len(s.Lhs) == len(s.Rhs) {
				for i, lhs := range s.Lhs {
					if hasAny(s.Rhs[i], v.loopVars) {
						v.loopVars = append(v.loopVars, lhs)
					}
				}
			} else if len(s.Rhs) == 1 && hasAny(s.Rhs[0], v.loopVars) {
				v.loopVars = append(v.loopVars, s.Lhs...)
			}
		}
		if len(s.Lhs) != len(s.Rhs) {
			return nil
		}
		for i, lhs := range s.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			if lenExpr := v.isCreateArray(s.Rhs[i]); lenExpr != nil {
				v.sliceDeclarations = append(v.sliceDeclarations, &sliceDeclaration{name: ident.Name, pos: s.Pos(), level: v.level, lenExpr: lenExpr})
			} else {
				declIdx := -1
				for i := len(v.sliceDeclarations) - 1; i >= 0; i-- {
					if v.sliceDeclarations[i].name == ident.Name {
						declIdx = i
						break
					}
				}
				if declIdx < 0 {
					continue
				}
				sliceDecl := v.sliceDeclarations[declIdx]
				switch expr := s.Rhs[i].(type) {
				case *ast.Ident:
					// create a new slice declaration when reinitializing an existing slice to nil
					if s.Tok == token.ASSIGN && expr.Name == "nil" {
						v.sliceDeclarations = append(v.sliceDeclarations, &sliceDeclaration{name: ident.Name, pos: s.Pos(), level: v.level, lenExpr: intExpr(0)})
						continue
					}
				case *ast.CallExpr:
					if len(expr.Args) >= 2 && !sliceDecl.hasReturn && sliceDecl.level == v.level {
						if funIdent, ok := expr.Fun.(*ast.Ident); ok && funIdent.Name == "append" {
							if rhsIdent, ok := expr.Args[0].(*ast.Ident); ok && ident.Name == rhsIdent.Name {
								sliceDecl.assigning = true
								continue
							}
						}
					}
				}
				sliceDecl.exclude = true
			}
		}

	case *ast.CallExpr:
		if funIdent, ok := s.Fun.(*ast.Ident); ok && funIdent.Name == "append" && len(s.Args) >= 2 {
			if rhsIdent, ok := s.Args[0].(*ast.Ident); ok {
				declIdx := -1
				for i := len(v.sliceDeclarations) - 1; i >= 0; i-- {
					if v.sliceDeclarations[i].name == rhsIdent.Name {
						declIdx = i
						break
					}
				}
				if declIdx < 0 {
					return v
				}
				sliceDecl := v.sliceDeclarations[declIdx]
				if sliceDecl.exclude {
					return v
				}

				if sliceDecl.hasReturn || sliceDecl.level != v.level || sliceDecl.detached {
					sliceDecl.exclude = true
					return v
				}

				countExpr := v.appendCount(s)
				if countExpr != nil && (hasAny(countExpr, v.loopVars) || hasVarReference(countExpr, sliceDecl.name)) {
					// exclude slice if append count references it
					sliceDecl.exclude = true
					return v
				}

				if sliceDecl.assigning {
					sliceDecl.assigning = false
				} else {
					sliceDecl.detached = true
				}
				v.sliceAppends = append(v.sliceAppends, &sliceAppend{index: declIdx, countExpr: countExpr})
			}
		}

	case *ast.RangeStmt:
		return v.walkRange(s)

	case *ast.ForStmt:
		return v.walkFor(s)

	case *ast.SwitchStmt:
		return v.walkSwitchSelect(s.Body)

	case *ast.TypeSwitchStmt:
		return v.walkSwitchSelect(s.Body)

	case *ast.SelectStmt:
		return v.walkSwitchSelect(s.Body)

	case *ast.ReturnStmt:
		if !v.simple {
			return nil
		}
		v.hasReturn = true
		// flag all slices that have been appended at least once
		for _, sliceApp := range v.sliceAppends {
			if sliceApp != nil {
				v.sliceDeclarations[sliceApp.index].hasReturn = true
			}
		}

	case *ast.BranchStmt:
		if !v.simple {
			return nil
		}
		if s.Label != nil {
			v.hasGoto = true
		} else {
			v.hasBranch = true
		}
	}

	return v
}

func (v *returnsVisitor) walkRange(stmt *ast.RangeStmt) ast.Visitor {
	if len(v.sliceDeclarations) == 0 {
		return v
	}
	if stmt.Body == nil {
		return nil
	}

	appendIdx := len(v.sliceAppends)
	hadBranch := v.hasBranch
	v.hasBranch = false
	v.level--
	varsIdx := len(v.loopVars)
	if stmt.Key != nil {
		v.loopVars = append(v.loopVars, stmt.Key)
	}
	if stmt.Value != nil {
		v.loopVars = append(v.loopVars, stmt.Value)
	}
	ast.Walk(v, stmt.Body)
	v.level++
	v.loopVars = v.loopVars[:varsIdx]

	exclude := !v.includeRangeLoops || v.hasReturn || v.hasGoto || v.hasBranch
	var loopCountExpr ast.Expr
	if !exclude {
		var ok bool
		loopCountExpr, ok = v.rangeLoopCount(stmt)
		exclude = !ok
	}
	if exclude {
		// exclude all slices that were appended within this loop
		for i := appendIdx; i < len(v.sliceAppends); i++ {
			if v.sliceAppends[i] != nil {
				v.sliceDeclarations[v.sliceAppends[i].index].exclude = true
			}
		}
	} else {
		for i, sliceDecl := range v.sliceDeclarations {
			if sliceDecl.exclude {
				continue
			}
			prev := -1
			for j := len(v.sliceAppends) - 1; j >= appendIdx; j-- {
				if v.sliceAppends[j] != nil && v.sliceAppends[j].index == i {
					if prev < 0 {
						if loopCountExpr == nil {
							// make appends indeterminate if the loop count is indeterminate
							v.sliceAppends[j].countExpr = nil
						} else if hasVarReference(loopCountExpr, sliceDecl.name) {
							// exclude slice if loop count references it
							sliceDecl.exclude = true
							break
						}
					} else {
						// consolidate appends to the same slice
						v.sliceAppends[j].countExpr = addIntExpr(v.sliceAppends[j].countExpr, v.sliceAppends[prev].countExpr)
						v.sliceAppends[prev] = nil
					}
					prev = j
				}
			}
			if prev >= 0 {
				v.sliceAppends[prev].countExpr = mulIntExpr(v.sliceAppends[prev].countExpr, loopCountExpr)
			}
		}
	}
	v.hasBranch = hadBranch
	return nil
}

func (v *returnsVisitor) walkFor(stmt *ast.ForStmt) ast.Visitor {
	if len(v.sliceDeclarations) == 0 {
		return v
	}
	if stmt.Body == nil {
		return nil
	}

	appendIdx := len(v.sliceAppends)
	hadBranch := v.hasBranch
	v.hasBranch = false
	v.level--
	varsIdx := len(v.loopVars)
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		v.loopVars = append(v.loopVars, assign.Lhs...)
	}
	ast.Walk(v, stmt.Body)
	v.level++
	v.loopVars = v.loopVars[:varsIdx]

	exclude := !v.includeForLoops || v.hasReturn || v.hasGoto || v.hasBranch
	var loopCountExpr ast.Expr
	if !exclude {
		var ok bool
		loopCountExpr, ok = v.forLoopCount(stmt)
		exclude = !ok
	}
	if exclude {
		// exclude all slices that were appended within this loop
		for i := appendIdx; i < len(v.sliceAppends); i++ {
			if v.sliceAppends[i] != nil {
				v.sliceDeclarations[v.sliceAppends[i].index].exclude = true
			}
		}
	} else {
		for i, sliceDecl := range v.sliceDeclarations {
			if sliceDecl.exclude {
				continue
			}
			prev := -1
			for j := len(v.sliceAppends) - 1; j >= appendIdx; j-- {
				if v.sliceAppends[j] != nil && v.sliceAppends[j].index == i {
					if prev < 0 {
						if loopCountExpr == nil {
							// make appends indeterminate if the loop count is indeterminate
							v.sliceAppends[j].countExpr = nil
						} else if hasVarReference(loopCountExpr, sliceDecl.name) {
							// exclude slice if loop count references it
							sliceDecl.exclude = true
							break
						}
					} else {
						// consolidate appends to the same slice
						v.sliceAppends[j].countExpr = addIntExpr(v.sliceAppends[j].countExpr, v.sliceAppends[prev].countExpr)
						v.sliceAppends[prev] = nil
					}
					prev = j
				}
			}
			if prev >= 0 {
				v.sliceAppends[prev].countExpr = mulIntExpr(v.sliceAppends[prev].countExpr, loopCountExpr)
			}
		}
	}
	v.hasBranch = hadBranch
	return nil
}

func (v *returnsVisitor) walkSwitchSelect(body *ast.BlockStmt) ast.Visitor {
	hadBranch := v.hasBranch
	v.hasBranch = false
	ast.Walk(v, body)
	v.hasBranch = hadBranch
	return nil
}

func (v *returnsVisitor) isCreateArray(expr ast.Expr) ast.Expr {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		// []any{...}
		if t := v.pass.TypesInfo.TypeOf(e); t != nil {
			switch t.Underlying().(type) {
			case *types.Array, *types.Slice:
				return intExpr(len(e.Elts))
			}
		}
	case *ast.CallExpr:
		switch len(e.Args) {
		case 1:
			// []any(nil)
			arg, ok := e.Args[0].(*ast.Ident)
			if !ok || arg.Name != "nil" {
				return nil
			}
			if t := v.pass.TypesInfo.TypeOf(e.Fun); t != nil {
				if _, ok := t.Underlying().(*types.Slice); ok {
					return intExpr(0)
				}
			}
		case 2:
			// make([]any, n)
			ident, ok := e.Fun.(*ast.Ident)
			if ok && ident.Name == "make" {
				return e.Args[1]
			}
		}
	}
	return nil
}

func (v *returnsVisitor) rangeLoopCount(stmt *ast.RangeStmt) (ast.Expr, bool) {
	x := stmt.X
	if hasAny(x, v.loopVars) {
		return nil, false
	}

	if call, ok := x.(*ast.CallExpr); ok {
		if len(call.Args) == 1 {
			if _, ok := call.Fun.(*ast.ArrayType); ok {
				x = call.Args[0]
			}
		} else if len(call.Args) >= 2 {
			if funIdent, ok := call.Fun.(*ast.Ident); ok && funIdent.Name == "append" {
				return addIntExpr(v.sliceLength(call.Args[0]), v.appendCount(call)), true
			}
		}
	}

	xType := v.pass.TypesInfo.TypeOf(x)
	if xType != nil {
		xType = xType.Underlying()
	}

	switch xType := xType.(type) {
	case *types.Chan, *types.Signature:
		return nil, false
	case *types.Array:
		if _, ok := stmt.X.(*ast.CompositeLit); ok && xType.Len() >= 0 {
			return intExpr(int(xType.Len())), true
		}
	case *types.Slice:
		if lit, ok := stmt.X.(*ast.CompositeLit); ok {
			return intExpr(len(lit.Elts)), true
		}
	case *types.Map:
		if lit, ok := x.(*ast.CompositeLit); ok {
			return intExpr(len(lit.Elts)), true
		}
	case *types.Pointer:
		if xType, ok := xType.Elem().(*types.Array); !ok {
			return nil, true
		} else if unary, ok := x.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if _, ok := unary.X.(*ast.CompositeLit); ok && xType.Len() >= 0 {
				return intExpr(int(xType.Len())), true
			}
		}
	case *types.Basic:
		if xType.Info()&types.IsString != 0 {
			if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if str, err := strconv.Unquote(lit.Value); err == nil {
					return intExpr(len(str)), true
				}
			}
		}
	default:
		return nil, true
	}

	if hasCall(x) {
		return nil, true
	}

	if xType, ok := xType.(*types.Basic); ok {
		switch {
		case xType.Info()&types.IsInteger != 0:
			return x, true
		case xType.Info()&types.IsString != 0:
		default:
			return nil, true
		}
	}

	if slice, ok := x.(*ast.SliceExpr); ok {
		high := slice.High
		if high == nil {
			high = &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{slice.X}}
		}
		if slice.Low != nil {
			return subIntExpr(high, slice.Low), true
		}
		return high, true
	}

	return &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{x}}, true
}

func (v *returnsVisitor) appendCount(expr *ast.CallExpr) ast.Expr {
	if expr.Ellipsis.IsValid() {
		return v.sliceLength(expr.Args[1])
	}
	return intExpr(len(expr.Args) - 1)
}

func (v *returnsVisitor) sliceLength(expr ast.Expr) ast.Expr {
	if call, ok := expr.(*ast.CallExpr); ok {
		if len(call.Args) == 1 {
			if _, ok := call.Fun.(*ast.ArrayType); ok {
				expr = call.Args[0]
			}
		} else if len(call.Args) >= 2 {
			if funIdent, ok := call.Fun.(*ast.Ident); ok && funIdent.Name == "append" {
				return addIntExpr(v.sliceLength(call.Args[0]), v.appendCount(call))
			}
		}
	}

	xType := v.pass.TypesInfo.TypeOf(expr)
	if xType == nil {
		return nil
	}
	switch xType := xType.Underlying().(type) {
	case *types.Array, *types.Slice:
		if lit, ok := expr.(*ast.CompositeLit); ok {
			return intExpr(len(lit.Elts))
		}
	case *types.Basic:
		if xType.Info()&types.IsString != 0 {
			if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if str, err := strconv.Unquote(lit.Value); err == nil {
					return intExpr(len(str))
				}
			}
		}
	default:
		return nil
	}

	if hasCall(expr) {
		return nil
	}

	if slice, ok := expr.(*ast.SliceExpr); ok {
		high := slice.High
		if high == nil {
			high = &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{slice.X}}
		}
		if slice.Low != nil {
			return subIntExpr(high, slice.Low)
		}
		return high
	}

	return &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{expr}}
}

func (v *returnsVisitor) forLoopCount(stmt *ast.ForStmt) (ast.Expr, bool) {
	if stmt.Init == nil || stmt.Cond == nil || stmt.Post == nil {
		return nil, false
	}

	if hasAny(stmt.Init, v.loopVars) {
		return nil, false
	}
	if hasAny(stmt.Cond, v.loopVars) {
		return nil, false
	}

	initAssign, ok := stmt.Init.(*ast.AssignStmt)
	if !ok || len(initAssign.Lhs) != len(initAssign.Rhs) {
		return nil, true
	}

	for i, lhs := range initAssign.Lhs {
		initIdent, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}

		var reverse bool
		var step ast.Expr
		switch s := stmt.Post.(type) {
		case *ast.IncDecStmt:
			if isIdentName(s.X, initIdent.Name) {
				reverse = s.Tok == token.DEC
				step = intExpr(1)
			}

		case *ast.AssignStmt:
			if len(s.Lhs) != len(s.Rhs) {
				return nil, true
			}

			for i, lhs := range s.Lhs {
				if !isIdentName(lhs, initIdent.Name) {
					continue
				}
				switch s.Tok {
				case token.ADD_ASSIGN, token.SUB_ASSIGN:
					step = s.Rhs[i]
					reverse = s.Tok == token.SUB_ASSIGN
				case token.ASSIGN:
					if rhsBinary, ok := s.Rhs[i].(*ast.BinaryExpr); ok {
						reverse = s.Tok == token.SUB
						if rhsBinary.Op == token.ADD || reverse {
							if isIdentName(rhsBinary.X, initIdent.Name) {
								step = rhsBinary.Y
							} else if isIdentName(rhsBinary.Y, initIdent.Name) {
								step = rhsBinary.X
							}
						}
					} else {
						return nil, false
					}
				default:
					return nil, false
				}
				if step != nil {
					break
				}
			}
		}

		if step == nil {
			continue
		}

		lower := initAssign.Rhs[i]
		if hasCall(lower) {
			continue // NATO: this should trigger another attempt
		}

		upper, op := forLoopUpperBound(stmt.Cond, initIdent.Name)

		if !reverse {
			if op == token.GTR || op == token.GEQ {
				return nil, false
			}
		} else {
			if op == token.LSS || op == token.LEQ {
				return nil, false
			}
			lower, upper = upper, lower
		}

		if op == token.LEQ || op == token.GEQ {
			upper = incIntExpr(upper)
		}

		countExpr, rounded := divIntExpr(subIntExpr(upper, lower), step)
		if rounded {
			// extra capacity in case non-unary step increment is rounded down
			countExpr = incIntExpr(countExpr)
		}
		return countExpr, true
	}

	return nil, true
}

func forLoopUpperBound(expr ast.Expr, name string) (ast.Expr, token.Token) {
	binExpr, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return nil, 0
	}

	switch binExpr.Op {
	case token.LAND, token.LOR:
		if xExpr, xOp := forLoopUpperBound(binExpr.X, name); xExpr != nil {
			if yExpr, yOp := forLoopUpperBound(binExpr.Y, name); yExpr != nil {
				if xOp == yOp {
					var funName string
					if binExpr.Op == token.LAND {
						funName = "min"
					} else {
						funName = "max"
					}
					if call, ok := xExpr.(*ast.CallExpr); ok {
						if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funName {
							call.Args = append(call.Args, yExpr)
							return xExpr, xOp
						}
					}
					if call, ok := yExpr.(*ast.CallExpr); ok {
						if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funName {
							call.Args = append(call.Args, xExpr)
							return yExpr, yOp
						}
					}
					return &ast.CallExpr{Fun: ast.NewIdent(funName), Args: []ast.Expr{xExpr, yExpr}}, xOp
				}
			}
		}

	case token.LSS, token.GTR, token.LEQ, token.GEQ, token.NEQ:
		if ident, ok := binExpr.X.(*ast.Ident); ok && ident.Name == name {
			if hasCall(binExpr.Y) {
				return nil, 0
			}
			return binExpr.Y, binExpr.Op
		} else if ident, ok := binExpr.Y.(*ast.Ident); ok && ident.Name == name {
			if hasCall(binExpr.X) {
				return nil, 0
			}
			// reverse the inequality
			op := binExpr.Op
			switch op {
			case token.LSS:
				op = token.GTR
			case token.GTR:
				op = token.LSS
			case token.LEQ:
				op = token.GEQ
			case token.GEQ:
				op = token.LEQ
			default:
			}
			return binExpr.X, op
		}

	default:
	}

	return nil, 0
}

func isIdentName(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func hasAny(node ast.Node, exprs []ast.Expr) bool {
	var found bool
	ast.Inspect(node, func(node ast.Node) bool {
		if expr, ok := node.(ast.Expr); ok {
			for _, e := range exprs {
				if exprEqual(expr, e) {
					found = true
					break
				}
			}
		}
		return !found
	})
	return found
}

func hasCall(expr ast.Expr) bool {
	var found bool
	ast.Inspect(expr, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			switch fun := call.Fun.(type) {
			case *ast.ArrayType, *ast.MapType:
				// allow array/map type convertion
				return true
			case *ast.Ident:
				switch fun.Name {
				case "bool", "error", "string", "any",
					"byte", "rune", "int", "int8", "int16", "int32", "int64",
					"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
					"float32", "float64", "complex64", "complex128":
					// allow built-in type conversions
					if len(call.Args) == 1 {
						return true
					}
				case "len", "cap", "real", "imag", "min", "max", "complex":
					// allow cheap pure built-in functions
					return true
				}
			case *ast.SelectorExpr:
				// allow argument-less methods
				if len(call.Args) == 0 {
					return true
				}
			}
			found = true
		}
		return !found
	})
	return found
}

func hasVarReference(expr ast.Expr, name string) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			// process target expression, ignore field selector
			found = hasVarReference(n.X, name)
			return false
		case *ast.CallExpr:
			// process args, ignore function name
			for _, arg := range n.Args {
				if found = hasVarReference(arg, name); found {
					break
				}
			}
			return false
		case *ast.KeyValueExpr:
			// process value, ignore key
			found = hasVarReference(n.Value, name)
			return false
		case *ast.Ident:
			found = n.Name == name
		}
		return !found
	})
	return found
}
