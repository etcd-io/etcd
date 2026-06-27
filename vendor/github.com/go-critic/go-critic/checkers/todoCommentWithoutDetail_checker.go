package checkers

import (
	"go/ast"
	"regexp"

	"github.com/go-critic/go-critic/checkers/internal/astwalk"
	"github.com/go-critic/go-critic/linter"
)

func init() {
	var info linter.CheckerInfo
	info.Name = "todoCommentWithoutDetail"
	info.Tags = []string{linter.StyleTag, linter.OpinionatedTag, linter.ExperimentalTag}
	info.Summary = "Detects TODO comments without detail/assignee"
	info.Before = `
// TODO
fiiWithCtx(nil, a, b)
`
	info.After = `
// TODO(admin): pass context.TODO() instead of nil
fiiWithCtx(nil, a, b)
`
	collection.AddChecker(&info, func(ctx *linter.CheckerContext) (linter.FileWalker, error) {
		visitor := &todoCommentWithoutCodeChecker{
			ctx:   ctx,
			regex: regexp.MustCompile(`^(//|/\*)?\s*(TODO|FIX|FIXME|BUG)\s*(\*/)?$`),
		}
		return astwalk.WalkerForComment(visitor), nil
	})
}

type todoCommentWithoutCodeChecker struct {
	astwalk.WalkHandler
	ctx   *linter.CheckerContext
	regex *regexp.Regexp
}

func (c *todoCommentWithoutCodeChecker) VisitComment(cg *ast.CommentGroup) {
	for _, comment := range cg.List {
		if c.regex.MatchString(comment.Text) {
			c.warn(cg)
			break
		}
	}
}

func (c *todoCommentWithoutCodeChecker) warn(cause ast.Node) {
	c.ctx.Warn(cause, "may want to add detail/assignee to this TODO/FIXME/BUG comment")
}
