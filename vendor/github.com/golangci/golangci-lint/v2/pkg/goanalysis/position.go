package goanalysis

import (
	"go/ast"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/analysis"
)

func GetGoFilePosition(pass *analysis.Pass, f *ast.File) (token.Position, bool) {
	position := GetFilePositionFor(pass.Fset, f.Pos())

	if filepath.Ext(position.Filename) == ".go" {
		return position, true
	}

	return position, false
}

func GetFilePositionFor(fset *token.FileSet, p token.Pos) token.Position {
	pos := fset.PositionFor(p, true)

	ext := filepath.Ext(pos.Filename)
	if ext != ".go" {
		// position has been adjusted to a non-go file, revert to original file
		return fset.PositionFor(p, false)
	}

	return pos
}

func EndOfLinePos(f *token.File, line int) token.Pos {
	var end token.Pos

	if line >= f.LineCount() {
		// missing newline at the end of the file
		end = f.Pos(f.Size())
	} else {
		end = f.LineStart(line+1) - token.Pos(1)
	}

	return end
}

// AdjustPos is a hack to get the right line to display.
// It should not be used outside some specific cases.
func AdjustPos(line, nonAdjLine, adjLine int) int {
	return line + nonAdjLine - adjLine
}
