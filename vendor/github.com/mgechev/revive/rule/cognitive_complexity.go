package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/mgechev/revive/lint"
)

// CognitiveComplexityRule sets restriction for maximum cognitive complexity.
type CognitiveComplexityRule struct {
	maxComplexity int
}

const defaultMaxCognitiveComplexity = 7

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *CognitiveComplexityRule) Configure(arguments lint.Arguments) error {
	if len(arguments) < 1 {
		r.maxComplexity = defaultMaxCognitiveComplexity
		return nil
	}

	complexity, ok := arguments[0].(int64)
	if !ok {
		return fmt.Errorf("invalid argument type for cognitive-complexity, expected int64, got %T", arguments[0])
	}

	r.maxComplexity = int(complexity)
	return nil
}

// Apply applies the rule to given file.
func (r *CognitiveComplexityRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	linter := cognitiveComplexityLinter{
		file:          file,
		maxComplexity: r.maxComplexity,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	linter.lintCognitiveComplexity()

	return failures
}

// Name returns the rule name.
func (*CognitiveComplexityRule) Name() string {
	return "cognitive-complexity"
}

type cognitiveComplexityLinter struct {
	file          *lint.File
	maxComplexity int
	onFailure     func(lint.Failure)
}

func (w cognitiveComplexityLinter) lintCognitiveComplexity() {
	f := w.file
	for _, decl := range f.AST.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			v := cognitiveComplexityVisitor{
				name: fn.Name,
			}
			c := v.subTreeComplexity(fn.Body)
			if c > w.maxComplexity {
				w.onFailure(lint.Failure{
					Confidence: 1,
					Category:   lint.FailureCategoryMaintenance,
					Failure:    fmt.Sprintf("function %s has cognitive complexity %d (> max enabled %d)", funcName(fn), c, w.maxComplexity),
					Node:       fn,
				})
			}
		}
	}
}

type cognitiveComplexityVisitor struct {
	name         *ast.Ident
	complexity   int
	nestingLevel int
}

// subTreeComplexity calculates the cognitive complexity of an AST-subtree.
func (v *cognitiveComplexityVisitor) subTreeComplexity(n ast.Node) int {
	ast.Walk(v, n)
	return v.complexity
}

// Visit implements the [ast.Visitor] interface.
func (v *cognitiveComplexityVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.IfStmt:
		v.walkIfElse(n)
		return nil
	case *ast.ForStmt:
		targets := []ast.Node{n.Cond, n.Body}
		v.walk(1, targets...)
		return nil
	case *ast.RangeStmt:
		v.walk(1, n.Body)
		return nil
	case *ast.SelectStmt:
		v.walk(1, n.Body)
		return nil
	case *ast.SwitchStmt:
		v.walk(1, n.Body)
		return nil
	case *ast.TypeSwitchStmt:
		v.walk(1, n.Body)
		return nil
	case *ast.FuncLit:
		v.walk(0, n.Body) // do not increment the complexity, just do the nesting
		return nil
	case *ast.BinaryExpr:
		v.complexity += v.binExpComplexity(n)
		return nil // skip visiting binexp subtree (already visited by binExpComplexity)
	case *ast.BranchStmt:
		if n.Label != nil {
			v.complexity++
		}
	case *ast.CallExpr:
		if ident, ok := n.Fun.(*ast.Ident); ok {
			if ident.Obj == v.name.Obj && ident.Name == v.name.Name {
				// called by same function directly (direct recursion)
				v.complexity++
				return nil
			}
		}
	}

	return v
}

func (v *cognitiveComplexityVisitor) walk(complexityIncrement int, targets ...ast.Node) {
	v.complexity += complexityIncrement + v.nestingLevel
	nesting := v.nestingLevel
	v.nestingLevel++

	for _, t := range targets {
		if t == nil {
			continue
		}

		ast.Walk(v, t)
	}

	v.nestingLevel = nesting
}

func (v *cognitiveComplexityVisitor) walkIfElse(n *ast.IfStmt) {
	var w func(n *ast.IfStmt)
	w = func(n *ast.IfStmt) {
		ast.Walk(v, n.Cond)
		ast.Walk(v, n.Body)
		if n.Else != nil {
			if elif, ok := n.Else.(*ast.IfStmt); ok {
				v.complexity++
				w(elif)
			} else {
				ast.Walk(v, n.Else)
			}
		}
	}

	// Nesting level is incremented in 'if' and 'else' blocks, but only the first 'if' in an 'if-else-if' chain sees its
	// complexity increased by the nesting level.
	v.complexity += 1 + v.nestingLevel
	v.nestingLevel++
	w(n)
	v.nestingLevel--
}

func (*cognitiveComplexityVisitor) binExpComplexity(n *ast.BinaryExpr) int {
	calculator := binExprComplexityCalculator{opsStack: []token.Token{}}

	astutil.Apply(n, calculator.pre, calculator.post)

	return calculator.complexity
}

type binExprComplexityCalculator struct {
	complexity    int
	opsStack      []token.Token // stack of bool operators
	subexpStarted bool
}

func (becc *binExprComplexityCalculator) pre(c *astutil.Cursor) bool {
	switch n := c.Node().(type) {
	case *ast.BinaryExpr:
		isBoolOp := n.Op == token.LAND || n.Op == token.LOR
		if !isBoolOp {
			break
		}

		ops := len(becc.opsStack)
		// if
		// 		is the first boolop in the expression OR
		// 		is the first boolop inside a subexpression (...) OR
		//		is not the same to the previous one
		// then
		//      increment complexity
		if ops == 0 || becc.subexpStarted || n.Op != becc.opsStack[ops-1] {
			becc.complexity++
			becc.subexpStarted = false
		}

		becc.opsStack = append(becc.opsStack, n.Op)
	case *ast.ParenExpr:
		becc.subexpStarted = true
	}

	return true
}

func (becc *binExprComplexityCalculator) post(c *astutil.Cursor) bool {
	switch n := c.Node().(type) {
	case *ast.BinaryExpr:
		isBoolOp := n.Op == token.LAND || n.Op == token.LOR
		if !isBoolOp {
			break
		}

		ops := len(becc.opsStack)
		if ops > 0 {
			becc.opsStack = becc.opsStack[:ops-1]
		}
	case *ast.ParenExpr:
		becc.subexpStarted = false
	}

	return true
}
