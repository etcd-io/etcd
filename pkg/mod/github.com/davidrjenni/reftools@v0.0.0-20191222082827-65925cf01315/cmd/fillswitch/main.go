// Copyright (c) 2017 David R. Jenni. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Fillswitch fills a (type) switch with case statements.
//
// For example, the following (type) switches,
//
//	var stmt ast.Stmt
//	switch stmt := stmt.(type) {
//	}
//
//	var kind ast.ObjKind
//	switch kind {
//	}
//
// become:
//
//	var stmt ast.Stmt
//	switch stmt := stmt.(type) {
//	case *ast.AssignStmt:
//	case *ast.BadStmt:
//	case *ast.BlockStmt:
//	case *ast.BranchStmt:
//	case *ast.CaseClause:
//	case *ast.CommClause:
//	case *ast.DeclStmt:
//	case *ast.DeferStmt:
//	case *ast.EmptyStmt:
//	case *ast.ExprStmt:
//	case *ast.ForStmt:
//	case *ast.GoStmt:
//	case *ast.IfStmt:
//	case *ast.IncDecStmt:
//	case *ast.LabeledStmt:
//	case *ast.RangeStmt:
//	case *ast.ReturnStmt:
//	case *ast.SelectStmt:
//	case *ast.SendStmt:
//	case *ast.SwitchStmt:
//	case *ast.TypeSwitchStmt:
//	}
//
//	var kind ast.ObjKind
//	switch kind {
//	case ast.Bad:
//	case ast.Con:
//	case ast.Fun:
//	case ast.Lbl:
//	case ast.Pkg:
//	case ast.Typ:
//	case ast.Var:
//	}
//
// after applying fillswitch for the (type) switch statements.
//
// Usage:
//
// 	% fillstruct [-modified] -file=<filename> -offset=<byte offset> -line=<line number>
//
// Flags:
//
// -file:     filename
//
// -modified: read an archive of modified files from stdin
//
// -offset:   byte offset of the (type) switch, optional if -line is present
//
// -line:     line number of the (type) switch, optional if -offset is present
//
// If -offset as well as -line are present, then the tool first uses the
// more specific offset information. If there was no (type) switch found
// at the given offset, then the line information is used.
//
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/refactor/importgraph"
)

var errNotFound = errors.New("no switch statement found")

func main() {
	log.SetFlags(0)
	log.SetPrefix("fillswitch: ")

	var (
		filename = flag.String("file", "", "filename")
		modified = flag.Bool("modified", false, "read an archive of modified files from stdin")
		offset   = flag.Int("offset", 0, "byte offset of the (type) switch, optional if -line is present")
		line     = flag.Int("line", 0, "line number of the (type) switch, optional if -offset is present")
	)
	flag.Parse()

	if (*offset == 0 && *line == 0) || *filename == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	path, err := absPath(*filename)
	if err != nil {
		log.Fatal(err)
	}

	lprog, err := load(path, *modified)
	if err != nil {
		log.Fatal(err)
	}

	if *offset > 0 {
		err = byOffset(lprog, path, *offset, os.Stdout)
		switch err {
		case nil:
			return
		case errNotFound:
			// try using line information
		default:
			log.Fatal(err)
		}
	}

	if *line > 0 {
		err = byLine(lprog, path, *line, os.Stdout)
		switch err {
		case nil:
			return
		default:
			log.Fatal(err)
		}
	}

	log.Fatal(errNotFound)
}

func absPath(filename string) (string, error) {
	eval, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return "", err
	}
	return filepath.Abs(eval)
}

func load(path string, modified bool) (*loader.Program, error) {
	ctx := &build.Default
	if modified {
		archive, err := buildutil.ParseOverlayArchive(os.Stdin)
		if err != nil {
			return nil, err
		}
		ctx = buildutil.OverlayContext(ctx, archive)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	pkg, err := buildutil.ContainingPackage(ctx, cwd, path)
	if err != nil {
		return nil, err
	}

	conf := &loader.Config{Build: ctx}
	allowErrors(conf)
	conf.ImportWithTests(pkg.ImportPath)

	_, rev, _ := importgraph.Build(ctx)
	for p := range rev.Search(pkg.ImportPath) {
		conf.ImportWithTests(p)
	}
	return conf.Load()
}

func allowErrors(lconf *loader.Config) {
	ctxt := *lconf.Build
	ctxt.CgoEnabled = false
	lconf.Build = &ctxt
	lconf.AllowErrors = true
	lconf.ParserMode = parser.AllErrors
	lconf.TypeChecker.Error = func(error) {}
}

func byOffset(lprog *loader.Program, path string, offset int, dst io.Writer) error {
	f, pkg, pos, err := findPos(lprog, path, offset)
	if err != nil {
		return err
	}

	swtch, typ, err := findSwitchStmt(f, pkg.Info, pos)
	if err != nil {
		return err
	}

	start := lprog.Fset.Position(swtch.Pos()).Offset
	end := lprog.Fset.Position(swtch.End()).Offset

	newSwtch := fillSwitch(pkg, lprog, swtch, typ)
	out, err := prepareOutput(newSwtch, start, end)
	if err != nil {
		return err
	}
	return json.NewEncoder(dst).Encode([]output{out})
}

func findPos(lprog *loader.Program, path string, offset int) (*ast.File, *loader.PackageInfo, token.Pos, error) {
	for _, pkg := range lprog.InitialPackages() {
		for _, f := range pkg.Files {
			if file := lprog.Fset.File(f.Pos()); file.Name() == path {
				if offset > file.Size() {
					return nil, nil, 0,
						fmt.Errorf("file size (%d) is smaller than given offset (%d)", file.Size(), offset)
				}
				return f, pkg, file.Pos(offset), nil
			}
		}
	}
	return nil, nil, 0, fmt.Errorf("could not find file %q", path)
}

func findSwitchStmt(f *ast.File, info types.Info, pos token.Pos) (ast.Stmt, types.Type, error) {
	path, _ := astutil.PathEnclosingInterval(f, pos, pos)
	for _, n := range path {
		switch n := n.(type) {
		case *ast.SwitchStmt:
			return n, info.Types[n.Tag].Type, nil

		case *ast.TypeSwitchStmt:
			switch stmt := n.Assign.(type) {
			case *ast.AssignStmt:
				return n, info.Types[stmt.Rhs[0].(*ast.TypeAssertExpr).X].Type, nil
			case *ast.ExprStmt:
				return n, info.Types[stmt.X.(*ast.TypeAssertExpr).X].Type, nil
			}
			return nil, nil, errors.New("invalid type switch")

		default:
			// continue
		}
	}
	return nil, nil, errNotFound
}

func byLine(lprog *loader.Program, path string, line int, dst io.Writer) (err error) {
	var f *ast.File
	var pkg *loader.PackageInfo
	for _, p := range lprog.InitialPackages() {
		for _, af := range p.Files {
			if file := lprog.Fset.File(af.Pos()); file.Name() == path {
				f = af
				pkg = p
			}
		}
	}
	if f == nil || pkg == nil {
		return fmt.Errorf("could not find file %q", path)
	}

	var outs []output
	ast.Inspect(f, func(n ast.Node) bool {
		switch swtch := n.(type) {
		case *ast.SwitchStmt:
			startLine := lprog.Fset.Position(swtch.Pos()).Line
			endLine := lprog.Fset.Position(swtch.End()).Line
			if !(startLine <= line && line <= endLine) {
				return true
			}

			start := lprog.Fset.Position(swtch.Pos()).Offset
			end := lprog.Fset.Position(swtch.End()).Offset
			newSwtch := fillSwitch(pkg, lprog, swtch, pkg.Info.Types[swtch.Tag].Type)

			var out output
			out, err = prepareOutput(newSwtch, start, end)
			if err != nil {
				return false
			}
			outs = append(outs, out)

		case *ast.TypeSwitchStmt:
			startLine := lprog.Fset.Position(swtch.Pos()).Line
			endLine := lprog.Fset.Position(swtch.End()).Line
			if !(startLine <= line && line <= endLine) {
				return true
			}

			var typ types.Type
			switch stmt := swtch.Assign.(type) {
			case *ast.AssignStmt:
				typ = pkg.Info.Types[stmt.Rhs[0].(*ast.TypeAssertExpr).X].Type
			case *ast.ExprStmt:
				typ = pkg.Info.Types[stmt.X.(*ast.TypeAssertExpr).X].Type
			default:
				return true
			}

			newSwtch := fillSwitch(pkg, lprog, swtch, typ)
			start := lprog.Fset.Position(swtch.Pos()).Offset
			end := lprog.Fset.Position(swtch.End()).Offset

			var out output
			out, err = prepareOutput(newSwtch, start, end)
			if err != nil {
				return false
			}
			outs = append(outs, out)

		default:
			return true
		}

		return false
	})
	if err != nil {
		return err
	}
	if len(outs) == 0 {
		return errNotFound
	}

	for i := len(outs)/2 - 1; i >= 0; i-- {
		opp := len(outs) - 1 - i
		outs[i], outs[opp] = outs[opp], outs[i]
	}

	return json.NewEncoder(dst).Encode(outs)
}

type output struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Code  string `json:"code"`
}

func prepareOutput(n ast.Node, start, end int) (output, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), n); err != nil {
		return output{}, err
	}
	return output{
		Start: start,
		End:   end,
		Code:  buf.String(),
	}, nil
}
