// Copyright 2018 The Go Doctor Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dataflow

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"sort"
	"strings"

	"github.com/godoctor/godoctor/analysis/cfg"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/loader"
)

// PrintDefUseDot prints a GraphViz DOT file showing the control flow graph with
// definition-use links superimposed.
//
// This is used by the debug refactoring.
func PrintDefUseDot(f io.Writer, fset *token.FileSet, info *loader.PackageInfo, cfg *cfg.CFG) {
	du := DefUse(cfg, info)

	fmt.Fprintf(f, `digraph mgraph {
mode="heir";
splines="ortho";

`)

	blocks := cfg.Blocks()
	cfg.Sort(blocks)

	// Assign a number to each CFG node/statement
	// List all vertices before listing edges connecting them
	stmtNum := map[ast.Stmt]uint{}
	lastNum := uint(0)
	for _, stmt := range blocks {
		lastNum++
		stmtNum[stmt] = lastNum
		fmt.Fprintf(f, "\ts%d [label=\"%s\"];\n",
			lastNum, printStmt(stmt, cfg, fset))
	}

	for _, from := range blocks {
		// Show control flow edges in black
		succs := cfg.Succs(from)
		cfg.Sort(succs)
		for _, to := range succs {
			fmt.Fprintf(f, "\ts%d -> s%d\n", stmtNum[from], stmtNum[to])
		}

		// Show def-use edges in red, dotted
		usedIn := []ast.Stmt{}
		for stmt := range du[from] {
			usedIn = append(usedIn, stmt)
		}
		cfg.Sort(usedIn)
		for _, stmt := range usedIn {
			varList := reachingVars(from, stmt, info)
			if len(varList) > 0 {
				fmt.Fprintf(f, "\ts%d -> s%d [xlabel=\"%s\",style=dotted,fontcolor=red,color=red]\n",
					stmtNum[from],
					stmtNum[stmt],
					strings.TrimSpace(varList))
			}
		}

	}

	fmt.Fprintf(f, "}\n")
}

func printStmt(stmt ast.Stmt, cfg *cfg.CFG, fset *token.FileSet) string {
	switch stmt {
	case cfg.Entry:
		return "ENTRY"
	case cfg.Exit:
		return "EXIT"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%s (line %d)\\n%s",
			astutil.NodeDescription(stmt),
			fset.Position(stmt.Pos()).Line,
			summarize(stmt, fset))
	}
}

func summarize(stmt ast.Stmt, fset *token.FileSet) string {
	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	switch x := stmt.(type) {
	case *ast.ForStmt:
		writer.WriteString("for ")
		printer.Fprint(writer, fset, x.Cond)
	case *ast.IfStmt:
		writer.WriteString("if ")
		printer.Fprint(writer, fset, x.Cond)
	case *ast.SelectStmt:
		writer.WriteString("select")
	case *ast.SwitchStmt:
		writer.WriteString("switch ")
		printer.Fprint(writer, fset, x.Tag)
	default:
		printer.Fprint(writer, fset, stmt)
	}
	writer.Flush()

	sourceCode := b.String()
	sourceCode = strings.Replace(sourceCode, "\r", " ", -1)
	sourceCode = strings.Replace(sourceCode, "\n", " ", -1)
	sourceCode = strings.Replace(sourceCode, "\t", "    ", -1)
	sourceCode = strings.Replace(sourceCode, "\"", "\\\"", -1)

	if len(sourceCode) > 30 {
		sourceCode = sourceCode[:27] + "..."
	}

	return sourceCode
}

func reachingVars(from, to ast.Stmt, info *loader.PackageInfo) string {
	asgt, updt, decl, _ := ReferencedVars([]ast.Stmt{from}, info)
	_, _, _, use := ReferencedVars([]ast.Stmt{to}, info)

	vars := map[string]struct{}{}
	for variable := range asgt {
		if _, used := use[variable]; used {
			vars[variable.Name()] = struct{}{}
		}
	}
	for variable := range updt {
		if _, used := use[variable]; used {
			vars[variable.Name()] = struct{}{}
		}
	}
	for variable := range decl {
		if _, used := use[variable]; used {
			vars[variable.Name()] = struct{}{}
		}
	}

	varList := []string{}
	for name := range vars {
		varList = append(varList, name)
	}
	sort.Sort(sort.StringSlice(varList))

	var b bytes.Buffer
	for _, name := range varList {
		b.WriteString(fmt.Sprintf(" %s", name))
	}
	return b.String()
}

// PrintLiveVarsDot prints a GraphViz DOT file showing the control flow graph
// with information about the liveness of local variables superimposed.
//
// This is used by the debug refactoring.
func PrintLiveVarsDot(f io.Writer, fset *token.FileSet, info *loader.PackageInfo, cfg *cfg.CFG) {
	liveIn, liveOut := LiveVars(cfg, info)

	fmt.Fprintf(f, `digraph mgraph {
mode="heir";
splines="ortho";

`)

	blocks := cfg.Blocks()
	cfg.Sort(blocks)

	// Assign a number to each CFG node/statement
	// List all vertices before listing edges connecting them
	stmtNum := map[ast.Stmt]uint{}
	lastNum := uint(0)
	for _, stmt := range blocks {
		liveInStr := toString(liveIn[stmt])
		liveOutStr := toString(liveOut[stmt])

		lastNum++
		stmtNum[stmt] = lastNum
		fmt.Fprintf(f, "\ts%d [label=\"%s\\nLiveIn: %s\\nLiveOut: %s\"];\n",
			lastNum, printStmt(stmt, cfg, fset), liveInStr, liveOutStr)
	}

	for _, from := range blocks {
		succs := cfg.Succs(from)
		cfg.Sort(succs)
		for _, to := range succs {
			fmt.Fprintf(f, "\ts%d -> s%d\n", stmtNum[from], stmtNum[to])
		}
	}

	fmt.Fprintf(f, "}\n")
}

func toString(set map[*types.Var]struct{}) string {
	list := []string{}
	for variable := range set {
		list = append(list, variable.Name())
	}
	sort.Sort(sort.StringSlice(list))

	var b bytes.Buffer
	first := true
	b.WriteString("{")
	for _, name := range list {
		if !first {
			b.WriteString(", ")
		}
		first = false
		b.WriteString(name)
	}
	b.WriteString("}")
	return b.String()
}
