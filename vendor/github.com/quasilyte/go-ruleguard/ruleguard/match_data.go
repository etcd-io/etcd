package ruleguard

import (
	"go/ast"

	"github.com/quasilyte/gogrep"
)

type matchData struct {
	match gogrep.MatchData
}

func (m matchData) Node() ast.Node { return m.match.Node }

func (m matchData) CaptureList() []gogrep.CapturedNode { return m.match.Capture }

func (m matchData) CapturedByName(name string) (ast.Node, bool) {
	return m.match.CapturedByName(name)
}
