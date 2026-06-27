package analyzer

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"dev.gaijin.team/go/exhaustruct/v4/internal/comment"
	"dev.gaijin.team/go/exhaustruct/v4/internal/structure"
)

type analyzer struct {
	config Config

	structFields structure.FieldsCache `exhaustruct:"optional"`
	comments     comment.Cache         `exhaustruct:"optional"`

	typeProcessingNeed   map[string]bool
	typeProcessingNeedMu sync.RWMutex `exhaustruct:"optional"`
}

func NewAnalyzer(config Config) (*analysis.Analyzer, error) {
	err := config.Prepare()
	if err != nil {
		return nil, err
	}

	a := analyzer{
		config:             config,
		typeProcessingNeed: make(map[string]bool),
		comments:           comment.Cache{},
	}

	return &analysis.Analyzer{ //nolint:exhaustruct
		Name:     "exhaustruct",
		Doc:      "Checks if all structure fields are initialized",
		Run:      a.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Flags:    *a.config.BindToFlagSet(flag.NewFlagSet("", flag.PanicOnError)),
	}, nil
}

func (a *analyzer) run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:forcetypeassert

	insp.WithStack([]ast.Node{(*ast.CompositeLit)(nil)}, a.newVisitor(pass))

	return nil, nil //nolint:nilnil
}

// newVisitor returns visitor that only expects [ast.CompositeLit] nodes.
func (a *analyzer) newVisitor(pass *analysis.Pass) func(n ast.Node, push bool, stack []ast.Node) bool {
	return func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}

		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			// this should never happen, but better be prepared
			return true
		}

		structTyp, typeInfo, ok := getStructType(pass, lit)
		if !ok {
			return true
		}

		if len(lit.Elts) == 0 && a.checkEmptyStructAllowed(pass, stack, typeInfo) {
			return true
		}

		file := a.comments.Get(pass.Fset, stack[0].(*ast.File)) //nolint:forcetypeassert
		rc := getCompositeLitRelatedComments(stack, file)
		pos, msg := a.processStruct(pass, lit, structTyp, typeInfo, rc)

		if pos != nil {
			pass.Reportf(*pos, "%s", msg)
		}

		return true
	}
}

func (a *analyzer) checkEmptyStructAllowed(pass *analysis.Pass, stack []ast.Node, typeInfo *TypeInfo) bool {
	// empty structs are globally allowed
	if a.config.AllowEmpty {
		return true
	}

	// some structs are allowed to be empty, basing on pattern
	if a.config.allowEmptyPatterns.MatchFullString(typeInfo.String()) {
		return true
	}

	if ret, ok := getParentReturnStmt(stack); ok {
		// empty structures are allowed in all return statements
		if a.config.AllowEmptyReturns {
			return true
		}

		// empty structures are allowed in error returns
		if isErrorReturnStatement(pass, ret, stack[len(stack)-1]) {
			return true
		}
	}

	// empty structures are allowed in variable declarations
	if isChildOfVariableDeclaration(stack) && a.config.AllowEmptyDeclarations {
		return true
	}

	return false
}

// isPartOfVariableDeclaration checks if the node is direct part of variable
// declaration, meaning that it is a first-level RHS child of `:=` or `var`
// declaration.
func isChildOfVariableDeclaration(stack []ast.Node) bool {
	if len(stack) < 2 { //nolint:mnd // stack for sure contains at leas current node and its parent (file)
		return false
	}

	// Start from composite literal and go up the stack
	for i := len(stack) - 1; i > 0; i-- {
		parent := stack[i-1]

		switch p := parent.(type) {
		case *ast.AssignStmt:
			if p.Tok == token.DEFINE {
				return true
			}

		case *ast.ValueSpec:
			return true

		case *ast.UnaryExpr:
			// Only allow pointer taking (&)
			if p.Op == token.AND {
				continue
			}

			return false

		default:
			return false
		}
	}

	return false
}

// getParentReturnStmt checks if the direct parent of the current node is a
// return statement and returns it if so.
func getParentReturnStmt(stack []ast.Node) (*ast.ReturnStmt, bool) {
	if len(stack) < 2 { //nolint:mnd // stack for sure contains at leas current node and its parent (file)
		return nil, false
	}

	// Start from composite literal and go up the stack
	for i := len(stack) - 1; i > 0; i-- {
		parent := stack[i-1]

		switch p := parent.(type) {
		case *ast.ReturnStmt:
			return p, true

		case *ast.UnaryExpr:
			// Only allow pointer taking (&)
			if p.Op == token.AND {
				continue
			}

			return nil, false

		default:
			return nil, false
		}
	}

	return nil, false
}

// errorIface is an interface type of the [error] interface.
//
//nolint:forcetypeassert,gochecknoglobals
var errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

// isErrorReturnStatement checks if the return statement is an error return
// statement, meaning that it contains a non-nil value that implements [error].
func isErrorReturnStatement(pass *analysis.Pass, n *ast.ReturnStmt, currentNode ast.Node) bool {
	if len(n.Results) == 0 {
		return false
	}

	// iterate backwards, since idiomatic position of error is at the end
	for i := len(n.Results) - 1; i >= 0; i-- {
		ri := n.Results[i]

		// Skip the current node, since it is already being checked
		if ri == currentNode {
			continue
		}

		switch ri := ri.(type) {
		case *ast.Ident:
			// Skip nil values
			if ri.Name == "nil" {
				continue
			}

		case *ast.UnaryExpr:
			// Current node might be under the unary expression
			if ri.X == currentNode {
				continue
			}
		}

		// Check if the type implements error interface
		resultType := pass.TypesInfo.TypeOf(ri)
		if resultType != nil && types.Implements(resultType, errorIface) {
			return true
		}
	}

	return false
}

// getCompositeLitRelatedComments returns all comments that are related to checked node. We
// have to traverse the stack manually as ast do not associate comments with
// [ast.CompositeLit].
func getCompositeLitRelatedComments(stack []ast.Node, cm ast.CommentMap) []*ast.CommentGroup {
	comments := make([]*ast.CommentGroup, 0)

	for i := len(stack) - 1; i >= 0; i-- {
		node := stack[i]

		switch tn := node.(type) {
		case *ast.CompositeLit:
			// comments on the lines prior to literal
			comments = append(comments, cm[node]...)
			// comments on the same line as literal type definition
			// worth noting that event "typeless" literals have a type
			comments = append(comments, cm[tn.Type]...)

		case *ast.ReturnStmt, // return ...
			*ast.IndexExpr,    // map[enum]...{...}[key]
			*ast.CallExpr,     // myfunc(map...)
			*ast.UnaryExpr,    // &map...
			*ast.AssignStmt,   // variable assignment (without var keyword)
			*ast.DeclStmt,     // var declaration, parent of *ast.GenDecl
			*ast.GenDecl,      // var declaration, parent of *ast.ValueSpec
			*ast.ValueSpec,    // var declaration
			*ast.KeyValueExpr: // field declaration
			comments = append(comments, cm[node]...)

		default:
			return comments
		}
	}

	return comments
}

func getStructType(pass *analysis.Pass, lit *ast.CompositeLit) (*types.Struct, *TypeInfo, bool) {
	switch typ := types.Unalias(pass.TypesInfo.TypeOf(lit)).(type) {
	case *types.Named: // named type
		if structTyp, ok := typ.Underlying().(*types.Struct); ok {
			pkg := typ.Obj().Pkg()
			ti := TypeInfo{
				Name:        typ.Obj().Name(),
				PackageName: pkg.Name(),
				PackagePath: pkg.Path(),
			}

			return structTyp, &ti, true
		}

		return nil, nil, false

	case *types.Struct: // anonymous struct
		ti := TypeInfo{
			Name:        "<anonymous>",
			PackageName: pass.Pkg.Name(),
			PackagePath: pass.Pkg.Path(),
		}

		return typ, &ti, true

	default:
		return nil, nil, false
	}
}

func (a *analyzer) processStruct(
	pass *analysis.Pass,
	lit *ast.CompositeLit,
	structTyp *types.Struct,
	info *TypeInfo,
	comments []*ast.CommentGroup,
) (*token.Pos, string) {
	shouldProcess := a.shouldProcessType(info)

	if shouldProcess && comment.HasDirective(comments, comment.DirectiveIgnore) {
		return nil, ""
	}

	if !shouldProcess && !comment.HasDirective(comments, comment.DirectiveEnforce) {
		return nil, ""
	}

	// unnamed structures are only defined in same package, along with types that has
	// prefix identical to current package name.
	isSamePackage := info.PackagePath == pass.Pkg.Path()

	if f := a.litSkippedFields(lit, structTyp, !isSamePackage); len(f) > 0 {
		pos := lit.Pos()

		if len(f) == 1 {
			return &pos, fmt.Sprintf("%s is missing field %s", info.ShortString(), f.String())
		}

		return &pos, fmt.Sprintf("%s is missing fields %s", info.ShortString(), f.String())
	}

	return nil, ""
}

// shouldProcessType returns true if type should be processed basing off include
// and exclude patterns, defined though constructor and\or flags.
func (a *analyzer) shouldProcessType(info *TypeInfo) bool {
	if len(a.config.includePatterns) == 0 && len(a.config.excludePatterns) == 0 {
		return true
	}

	name := info.String()

	a.typeProcessingNeedMu.RLock()
	res, ok := a.typeProcessingNeed[name]
	a.typeProcessingNeedMu.RUnlock()

	if !ok {
		a.typeProcessingNeedMu.Lock()

		res = true

		if a.config.includePatterns != nil && !a.config.includePatterns.MatchFullString(name) {
			res = false
		}

		if res && a.config.excludePatterns != nil && a.config.excludePatterns.MatchFullString(name) {
			res = false
		}

		a.typeProcessingNeed[name] = res
		a.typeProcessingNeedMu.Unlock()
	}

	return res
}

func (a *analyzer) litSkippedFields(
	lit *ast.CompositeLit,
	typ *types.Struct,
	onlyExported bool,
) structure.Fields {
	return a.structFields.Get(typ).Skipped(lit, onlyExported)
}

type TypeInfo struct {
	Name        string
	PackageName string
	PackagePath string
}

func (t TypeInfo) String() string {
	return t.PackagePath + "." + t.Name
}

func (t TypeInfo) ShortString() string {
	return t.PackageName + "." + t.Name
}
