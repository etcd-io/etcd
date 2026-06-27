package rule

import (
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/lint"
)

// EmptyLinesRule lints empty lines in blocks.
type EmptyLinesRule struct{}

// Apply applies the rule to given file.
func (r *EmptyLinesRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := lintEmptyLines{file, r.commentLines(file.CommentMap(), file), onFailure}
	ast.Walk(w, file.AST)
	return failures
}

// Name returns the rule name.
func (*EmptyLinesRule) Name() string {
	return "empty-lines"
}

type lintEmptyLines struct {
	file      *lint.File
	cmap      map[int]struct{}
	onFailure func(lint.Failure)
}

func (w lintEmptyLines) Visit(node ast.Node) ast.Visitor {
	block, ok := node.(*ast.BlockStmt)
	if !ok || len(block.List) == 0 {
		return w
	}

	w.checkStart(block)
	w.checkEnd(block)

	return w
}

func (w lintEmptyLines) checkStart(block *ast.BlockStmt) {
	blockStart := w.position(block.Lbrace)
	firstNode := block.List[0]
	firstStmt := w.position(firstNode.Pos())

	firstBlockLineIsStmt := firstStmt.Line-(blockStart.Line+1) <= 0
	_, firstBlockLineIsComment := w.cmap[blockStart.Line+1]
	if firstBlockLineIsStmt || firstBlockLineIsComment {
		return
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       block,
		Category:   lint.FailureCategoryStyle,
		Failure:    "extra empty line at the start of a block",
	})
}

func (w lintEmptyLines) checkEnd(block *ast.BlockStmt) {
	blockEnd := w.position(block.Rbrace)
	lastNode := block.List[len(block.List)-1]
	lastStmt := w.position(lastNode.End())

	lastBlockLineIsStmt := (blockEnd.Line-1)-lastStmt.Line <= 0
	_, lastBlockLineIsComment := w.cmap[blockEnd.Line-1]
	if lastBlockLineIsStmt || lastBlockLineIsComment {
		return
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       block,
		Category:   lint.FailureCategoryStyle,
		Failure:    "extra empty line at the end of a block",
	})
}

func (w lintEmptyLines) position(pos token.Pos) token.Position {
	return w.file.ToPosition(pos)
}

func (*EmptyLinesRule) commentLines(cmap ast.CommentMap, file *lint.File) map[int]struct{} {
	result := map[int]struct{}{}

	for _, comments := range cmap {
		for _, comment := range comments {
			start := file.ToPosition(comment.Pos())
			end := file.ToPosition(comment.End())
			for i := start.Line; i <= end.Line; i++ {
				result[i] = struct{}{}
			}
		}
	}

	return result
}
