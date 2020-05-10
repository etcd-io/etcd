// Copyright 2015-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE File.

// This File defines a "debug" refactoring, which is not really a refactoring
// at all.  It does not change any files; rather, it is invoked to print
// information about the Go refactoring engine and its internals.  For example,
// it can display the AST for a File, output a GraphViz DOT File with a File's
// control flow graphs, display what package(s) are loaded, or display what
// identifiers resolve to what objects.

package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
	"github.com/godoctor/godoctor/analysis/names"
)

const usage = `Usage: debug <options>
where <options> can be any or all of the following:

Information about the entire file:
    showast           Show the abstract syntax tree for the selected file
    showidentifiers   Show name references (ast.Object) in initial packages
    showpackages      List all packages loaded (due to --scope)

If anything is selected:
    fmt               Format the node enclosing the selection using go/printer

If an identifier is selected:
    showaffected      Show names affected if the selected identifier is renamed
    showreferences    Show all direct references to the selected identifier

If a function is selected...
    showcfg           Output the control flow graph (CFG) in GraphViz DOT format
    showdefuse        Output CFG with def-use information GraphViz DOT format
    showlive          Output CFG with liveness information GraphViz DOT format

Use GraphViz's "dotty" tool to view DOT files.  For example:
    $ godoctor -file main.go -pos 5,1:5,1 debug showdefuse > output.dot
    $ dotty output.dot`

type Debug struct {
	RefactoringBase
}

func (r *Debug) Description() *Description {
	return &Description{
		Name:      "Debug Refactoring",
		Synopsis:  "Provides assorted debugging outputs",
		Usage:     "<command>",
		HTMLDoc:   "",
		Multifile: false,
		Params:    nil,
		OptionalParams: []Parameter{{
			Label:        "Command",
			Prompt:       "Command",
			DefaultValue: "",
		}},
		Hidden: true,
	}
}

func (r *Debug) Run(config *Config) *Result {
	r.Init(config, r.Description())

	if len(config.Args) == 0 {
		r.Log.Error(usage)
		return &r.Result
	}

	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}

	command := strings.ToLower(strings.TrimSpace(config.Args[0].(string)))

	switch command {
	case "fmt":
		r.fmt()
	case "showaffected":
		r.showAffected(&r.DebugOutput)
	case "showast":
		r.showAST(&r.DebugOutput)
	case "showcfg":
		r.showCFG(&r.DebugOutput)
	case "showdefuse":
		r.showDefUse(&r.DebugOutput)
	case "showlive":
		r.showLiveVars(&r.DebugOutput)
	case "showidentifiers":
		r.showIdentifiers(&r.DebugOutput)
	case "showpackages":
		r.showLoadedPackagesAndFiles(&r.DebugOutput)
	case "showreferences":
		r.showReferences(&r.DebugOutput)
	default:
		r.Log.Errorf("Unknown option %s", command)
	}
	return &r.Result
}

func (r *Debug) fmt() {
	// Find the smallest formattable node enclosing the selection
	_, nodes, _ := r.Program.PathEnclosingInterval(r.SelectionStart, r.SelectionEnd)
	for _, node := range nodes {
		if canFormat(node) {
			cnode := &printer.CommentedNode{
				Node:     node,
				Comments: r.File.Comments}
			printConfig := &printer.Config{
				Mode:     printer.UseSpaces | printer.TabIndent,
				Tabwidth: 8}
			var b bytes.Buffer
			err := printConfig.Fprint(&b, r.Program.Fset, cnode)
			if err != nil {
				r.Log.Error(err)
				return
			}

			extent := r.Extent(node)
			if r.goPrinterIncludesTrailingComments() && len(nodes) == 1 {
				// We are formatting the entire file, but the
				// extent stops before end-of-file comments.
				// Extend it so comments are not duplicated.
				extent.Length = len(r.FileContents)
			}
			r.Edits[r.Filename].Add(extent, b.String())
			return
		}
	}
}

func canFormat(node interface{}) bool {
	switch node.(type) {
	case ast.Expr:
		return true
	case ast.Stmt:
		return true
	case ast.Spec:
		return true
	case ast.Decl:
		return true
	case *ast.File:
		return true
	default:
		return false
	}

}

// The behavior of go/printer changed between Go 1.8 and Go 1.10: In Go 1.10,
// comments at the end of the file are included.  In Go 1.8, they were not.
// This detects whether trailing comments are included, so the refactoring can
// decide whether or not to include them in the region that is replaced.
func (r *Debug) goPrinterIncludesTrailingComments() bool {
	// Parse a simple file with a comment at the end
	str := "package main\n\n// END\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", str, parser.ParseComments)
	if err != nil {
		// r.Log.Error(err)
		return false
	}

	// Print it using go/printer
	cnode := &printer.CommentedNode{
		Node:     f,
		Comments: f.Comments}
	printConfig := &printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8}
	var b bytes.Buffer
	err = printConfig.Fprint(&b, fset, cnode)
	if err != nil {
		// r.Log.Error(err)
		return false
	}

	// Check if the trailing comment was included
	return strings.HasSuffix(b.String(), "END\n")
}

func (r *Debug) showAffected(out io.Writer) {
	errorMsg := "Please select an identifier for showaffected"

	if r.SelectedNode == nil {
		r.Log.Error(errorMsg)
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return
	}
	switch id := r.SelectedNode.(type) {
	case *ast.Ident:
		fmt.Fprintf(out, "Affected Declarations:\n")
		obj := r.SelectedNodePkg.ObjectOf(id)
		searchResult := names.FindDeclarationsAcrossInterfaces(obj, r.Program)
		result := []string{}
		for obj := range searchResult {
			p := r.Program.Fset.Position(obj.Pos())
			result = append(result,
				fmt.Sprintf("  %s - %s, Line %d\n",
					obj.Name(), p.Filename, p.Line))
		}
		sort.Strings(result)
		for _, line := range result {
			fmt.Fprintf(out, line)
		}
	default:
		r.Log.Error(errorMsg)
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return
	}
}

func (r *Debug) showAST(out io.Writer) {
	ast.Fprint(out, r.Program.Fset, r.File, nil)
}

func (r *Debug) showCFG(out io.Writer) {
	switch funcDecl := r.SelectedNode.(type) {
	case *ast.FuncDecl:
		if funcDecl.Name != nil {
			fmt.Fprintf(out, "// Control flow graph for %s\n", funcDecl.Name.Name)
		} else {
			fmt.Fprintf(out, "// Control flow graph for anonymous function\n")
		}
		cfg := cfg.FromFunc(funcDecl)
		cfg.PrintDot(out, r.Program.Fset, r.describeVariables)

	default:
		r.Log.Error("Please select a function.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		r.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectedNode.Pos(), r.SelectedNode.Pos())
	}
}

func (r *Debug) describeVariables(stmt ast.Stmt) string {
	var buf bytes.Buffer
	asgts, updts, decls, uses := dataflow.ReferencedVars([]ast.Stmt{stmt}, r.SelectedNodePkg)
	if len(asgts) > 0 {
		fmt.Fprintf(&buf, "Assigns: %s\n", listNames(asgts))
	}
	if len(updts) > 0 {
		fmt.Fprintf(&buf, "Updates: %s\n", listNames(updts))
	}
	if len(decls) > 0 {
		fmt.Fprintf(&buf, "Declares: %s\n", listNames(decls))
	}
	if len(uses) > 0 {
		fmt.Fprintf(&buf, "Uses: %s\n", listNames(uses))
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func listNames(vars map[*types.Var]struct{}) string {
	names := []string{}
	for v := range vars {
		names = append(names, v.Name())
	}
	sort.Strings(names)

	var buf bytes.Buffer
	for _, name := range names {
		fmt.Fprintf(&buf, ", %s", name)
	}
	return strings.TrimPrefix(buf.String(), ", ")
}

func (r *Debug) showDefUse(out io.Writer) {
	switch funcDecl := r.SelectedNode.(type) {
	case *ast.FuncDecl:
		if funcDecl.Name != nil {
			fmt.Fprintf(out, "// Def-use information for %s\n", funcDecl.Name.Name)
		} else {
			fmt.Fprintf(out, "// Def-use information for anonymous function\n")
		}
		cfg := cfg.FromFunc(funcDecl)
		dataflow.PrintDefUseDot(out, r.Program.Fset, r.SelectedNodePkg, cfg)

	default:
		r.Log.Error("Please select a function.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		r.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectedNode.Pos(), r.SelectedNode.Pos())
	}
}

func (r *Debug) showLiveVars(out io.Writer) {
	switch funcDecl := r.SelectedNode.(type) {
	case *ast.FuncDecl:
		if funcDecl.Name != nil {
			fmt.Fprintf(out, "// Live variables in %s\n", funcDecl.Name.Name)
		} else {
			fmt.Fprintf(out, "// Live variables in anonymous function\n")
		}
		cfg := cfg.FromFunc(funcDecl)
		dataflow.PrintLiveVarsDot(out, r.Program.Fset, r.SelectedNodePkg, cfg)

	default:
		r.Log.Error("Please select a function.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		r.Log.Errorf("(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectedNode.Pos(), r.SelectedNode.Pos())
	}
}

func (r *Debug) showIdentifiers(out io.Writer) {
	for _, pkgInfo := range r.Program.InitialPackages() {
		for _, file := range pkgInfo.Files {
			r.showIdentifiersInFile(pkgInfo, file, out)
		}
	}
}

func (r *Debug) showIdentifiersInFile(pkgInfo *loader.PackageInfo, file *ast.File, out io.Writer) {
	filename, err := r.getRelativeFilename(file)
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Fprintf(out, "=====%s=====\n",
		filename)
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		position := r.Program.Fset.Position(id.Pos())
		fmt.Fprintf(out, "%s\t(Line %d)", id.Name, position.Line)
		if obj := pkgInfo.ObjectOf(id); obj == nil {
			fmt.Fprintf(out, " does not reference an object\n")
		} else {
			fmt.Fprintf(out, " is a reference to %s (%s)\n",
				obj.Id(), r.Program.Fset.Position(obj.Pos()))
		}
		return true
	})
}

func (r *Debug) getRelativeFilename(file *ast.File) (string, error) {
	filename := r.Program.Fset.Position(file.Pos()).Filename
	cwd, err := os.Getwd()
	if err == nil {
		filename, err = filepath.Rel(cwd, filename)
	}
	return filename, err
}

func (r *Debug) showLoadedPackagesAndFiles(out io.Writer) {
	cwd, _ := os.Getwd()
	fmt.Fprintf(out, "Working directory is %s\n", cwd)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Packages/files loaded:")
	for _, pkgInfo := range r.Program.AllPackages {
		fmt.Fprintf(out, "\t%s\n", pkgInfo.Pkg.Name())
		for _, file := range pkgInfo.Files {
			filename := r.Program.Fset.Position(file.Pos()).Filename
			fmt.Fprintf(out, "\t\t%s\n", filename)
		}
	}
}

func (r *Debug) showReferences(out io.Writer) {
	errorMsg := "Please select an identifier for showreferences"

	if r.SelectedNode == nil {
		r.Log.Error(errorMsg)
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return
	}
	switch id := r.SelectedNode.(type) {
	case *ast.Ident:
		fmt.Fprintf(out, "References to %s:\n", id.Name)
		ids := names.FindOccurrences(r.SelectedNodePkg.ObjectOf(id), r.Program)
		strs := []string{}
		for id := range ids {
			description := fmt.Sprintf("  %s: %s",
				r.Program.Fset.Position(id.Pos()).Filename,
				r.Extent(id).String())
			strs = append(strs, description)
		}
		sort.Strings(strs)
		for _, s := range strs {
			fmt.Fprintln(out, s)
		}
	default:
		r.Log.Error(errorMsg)
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return
	}
}
