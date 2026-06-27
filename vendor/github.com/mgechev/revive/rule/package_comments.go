package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"sync"

	"github.com/mgechev/revive/lint"
)

// PackageCommentsRule lints the package comments. It complains if
// there is no package comment, or if it is not of the right form.
// This has a notable false positive in that a package comment
// could rightfully appear in a different file of the same package,
// but that's not easy to fix since this linter is file-oriented.
type PackageCommentsRule struct {
	checkPackageCommentCache sync.Map
}

// Apply applies the rule to given file.
func (r *PackageCommentsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	if file.IsTest() {
		return failures
	}

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	fileAst := file.AST
	w := &lintPackageComments{fileAst, file, onFailure, r}
	ast.Walk(w, fileAst)
	return failures
}

// Name returns the rule name.
func (*PackageCommentsRule) Name() string {
	return "package-comments"
}

type lintPackageComments struct {
	fileAst   *ast.File
	file      *lint.File
	onFailure func(lint.Failure)
	rule      *PackageCommentsRule
}

func (l *lintPackageComments) checkPackageComment() []lint.Failure {
	// deduplicate warnings in package
	if _, exists := l.rule.checkPackageCommentCache.LoadOrStore(l.file.Pkg, struct{}{}); exists {
		return nil
	}
	var docFile *ast.File     // which name is doc.go
	var packageFile *ast.File // which name is $package.go
	var firstFile *ast.File
	var firstFileName string
	var fileSource string
	for name, file := range l.file.Pkg.Files() {
		if file.AST.Doc != nil {
			return nil
		}
		if name == "doc.go" {
			docFile = file.AST
			fileSource = "doc.go"
		}
		if name == file.AST.Name.String()+".go" {
			packageFile = file.AST
		}
		if firstFileName == "" || firstFileName > name {
			firstFile = file.AST
			firstFileName = name
		}
	}
	// prefer warning on doc.go, $package.go over first file
	if docFile == nil {
		docFile = packageFile
		fileSource = l.fileAst.Name.String() + ".go"
	}
	if docFile == nil {
		docFile = firstFile
		fileSource = firstFileName
	}

	if docFile != nil {
		pkgFile := l.file.Pkg.Files()[fileSource]
		return []lint.Failure{{
			Category: lint.FailureCategoryComments,
			Position: lint.FailurePosition{
				Start: pkgFile.ToPosition(docFile.Pos()),
				End:   pkgFile.ToPosition(docFile.Name.End()),
			},
			Confidence: 1,
			Failure:    "should have a package comment",
		}}
	}
	return nil
}

func (l *lintPackageComments) Visit(_ ast.Node) ast.Visitor {
	if l.file.IsTest() {
		return nil
	}

	prefix := "Package " + l.fileAst.Name.Name + " "

	// Look for a detached package comment.
	// First, scan for the last comment that occurs before the "package" keyword.
	var lastCG *ast.CommentGroup
	for _, cg := range l.fileAst.Comments {
		if cg.Pos() > l.fileAst.Package {
			// Gone past "package" keyword.
			break
		}
		lastCG = cg
	}
	if lastCG != nil && strings.HasPrefix(lastCG.Text(), prefix) {
		endPos := l.file.ToPosition(lastCG.End())
		pkgPos := l.file.ToPosition(l.fileAst.Package)
		if endPos.Line+1 < pkgPos.Line {
			// There isn't a great place to anchor this error;
			// the start of the blank lines between the doc and the package statement
			// is at least pointing at the location of the problem.
			pos := token.Position{
				Filename: endPos.Filename,
				// Offset not set; it is non-trivial, and doesn't appear to be needed.
				Line:   endPos.Line + 1,
				Column: 1,
			}
			l.onFailure(lint.Failure{
				Category: lint.FailureCategoryComments,
				Position: lint.FailurePosition{
					Start: pos,
					End:   pos,
				},
				Confidence: 0.9,
				Failure:    "package comment is detached; there should be no blank lines between it and the package statement",
			})
			return nil
		}
	}

	if isEmptyDoc(l.fileAst.Doc) {
		for _, failure := range l.checkPackageComment() {
			l.onFailure(failure)
		}
		return nil
	}
	s := l.fileAst.Doc.Text()

	// Only non-main packages need to keep to this form.
	if !l.file.Pkg.IsMain() && !strings.HasPrefix(s, prefix) && !isDirectiveComment(s) {
		l.onFailure(lint.Failure{
			Category:   lint.FailureCategoryComments,
			Node:       l.fileAst.Doc,
			Confidence: 1,
			Failure:    fmt.Sprintf(`package comment should be of the form "%s..."`, prefix),
		})
	}
	return nil
}

func isEmptyDoc(commentGroup *ast.CommentGroup) bool {
	return commentGroup == nil || commentGroup.Text() == ""
}
