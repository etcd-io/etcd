package lintcmd

import (
	"strings"

	"honnef.co/go/tools/lintcmd/runner"
)

func parseDirectives(dirs []runner.SerializedDirective) ([]ignore, []diagnostic) {
	var ignores []ignore
	var diagnostics []diagnostic

	for _, dir := range dirs {
		cmd := dir.Command
		args := dir.Arguments
		switch cmd {
		case "ignore", "file-ignore":
			if len(args) < 2 {
				p := diagnostic{
					Diagnostic: runner.Diagnostic{
						Position: dir.NodePosition,
						Message:  "malformed linter directive; missing the required reason field?",
						Category: "compile",
					},
					Severity: severityError,
				}
				diagnostics = append(diagnostics, p)
				continue
			}
		default:
			// unknown directive, ignore
			continue
		}
		checks := strings.Split(args[0], ",")
		pos := dir.NodePosition
		var ig ignore
		switch cmd {
		case "ignore":
			ig = &lineIgnore{
				File:   pos.Filename,
				Line:   pos.Line,
				Checks: checks,
				Pos:    dir.DirectivePosition,
			}
		case "file-ignore":
			ig = &fileIgnore{
				File:   pos.Filename,
				Checks: checks,
			}
		}
		ignores = append(ignores, ig)
	}

	return ignores, diagnostics
}
