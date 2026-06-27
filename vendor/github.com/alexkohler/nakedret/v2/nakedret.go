package nakedret

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const pwd = "./"

func NakedReturnAnalyzer(nakedRet *NakedReturnRunner) *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name:     "nakedret",
		Doc:      "Checks that functions with naked returns are not longer than a maximum size (can be zero).",
		Run:      nakedRet.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}

	return a
}

type NakedReturnRunner struct {
	MaxLength     uint
	SkipTestFiles bool
}

func (n *NakedReturnRunner) run(pass *analysis.Pass) (any, error) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{ // filter needed nodes: visit only them
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.ReturnStmt)(nil),
	}
	retVis := &returnsVisitor{
		pass:          pass,
		f:             pass.Fset,
		maxLength:     n.MaxLength,
		skipTestFiles: n.SkipTestFiles,
	}
	inspector.Nodes(nodeFilter, retVis.NodesVisit)
	return nil, nil
}

type returnsVisitor struct {
	pass          *analysis.Pass
	f             *token.FileSet
	maxLength     uint
	skipTestFiles bool

	// functions contains funcInfo for each nested function definition encountered while visiting the AST.
	functions []funcInfo
}

type funcInfo struct {
	// Details of the function we're currently dealing with
	funcType    *ast.FuncType
	funcName    string
	funcLength  int
	reportNaked bool
}

func checkNakedReturns(args []string, maxLength *uint, skipTestFiles bool, setExitStatus bool) error {

	fset := token.NewFileSet()

	files, err := parseInput(args, fset)
	if err != nil {
		return fmt.Errorf("could not parse input: %v", err)
	}

	if maxLength == nil {
		return errors.New("max length nil")
	}

	analyzer := NakedReturnAnalyzer(&NakedReturnRunner{MaxLength: *maxLength, SkipTestFiles: skipTestFiles})
	pass := &analysis.Pass{
		Analyzer: analyzer,
		Fset:     fset,
		Files:    files,
		Report: func(d analysis.Diagnostic) {
			log.Printf("%s:%d: %s", fset.Position(d.Pos).Filename, fset.Position(d.Pos).Line, d.Message)
		},
		ResultOf: map[*analysis.Analyzer]any{},
	}
	result, err := inspect.Analyzer.Run(pass)
	if err != nil {
		return err
	}
	pass.ResultOf[inspect.Analyzer] = result

	_, err = analyzer.Run(pass)
	if err != nil {
		return err
	}

	return nil
}

func parseInput(args []string, fset *token.FileSet) ([]*ast.File, error) {
	var directoryList []string
	var fileMode bool
	files := make([]*ast.File, 0)

	if len(args) == 0 {
		directoryList = append(directoryList, pwd)
	} else {
		for _, arg := range args {
			if strings.HasSuffix(arg, "/...") && isDir(arg[:len(arg)-len("/...")]) {

				for _, dirname := range allPackagesInFS(arg) {
					directoryList = append(directoryList, dirname)
				}

			} else if isDir(arg) {
				directoryList = append(directoryList, arg)

			} else if exists(arg) {
				if strings.HasSuffix(arg, ".go") {
					fileMode = true
					f, err := parser.ParseFile(fset, arg, nil, 0)
					if err != nil {
						return nil, err
					}
					files = append(files, f)
				} else {
					return nil, fmt.Errorf("invalid file %v specified", arg)
				}
			} else {

				// TODO clean this up a bit
				imPaths := importPaths([]string{arg})
				for _, importPath := range imPaths {
					pkg, err := build.Import(importPath, ".", 0)
					if err != nil {
						return nil, err
					}
					var stringFiles []string
					stringFiles = append(stringFiles, pkg.GoFiles...)
					// files = append(files, pkg.CgoFiles...)
					stringFiles = append(stringFiles, pkg.TestGoFiles...)
					if pkg.Dir != "." {
						for i, f := range stringFiles {
							stringFiles[i] = filepath.Join(pkg.Dir, f)
						}
					}

					fileMode = true
					for _, stringFile := range stringFiles {
						f, err := parser.ParseFile(fset, stringFile, nil, 0)
						if err != nil {
							return nil, err
						}
						files = append(files, f)
					}

				}
			}
		}
	}

	// if we're not in file mode, then we need to grab each and every package in each directory
	// we can to grab all the files
	if !fileMode {
		for _, fpath := range directoryList {
			pkgs, err := parser.ParseDir(fset, fpath, nil, 0)
			if err != nil {
				return nil, err
			}

			for _, pkg := range pkgs {
				for _, f := range pkg.Files {
					files = append(files, f)
				}
			}
		}
	}

	return files, nil
}

func isDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

func exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func hasNamedReturns(funcType *ast.FuncType) bool {
	if funcType == nil || funcType.Results == nil {
		return false
	}
	for _, field := range funcType.Results.List {
		for _, ident := range field.Names {
			if ident != nil {
				return true
			}
		}
	}
	return false
}

func nestedFuncName(functions []funcInfo) string {
	var names []string
	for _, f := range functions {
		names = append(names, f.funcName)
	}
	return strings.Join(names, ".")
}

func nakedReturnFix(s *ast.ReturnStmt, funcType *ast.FuncType) *ast.ReturnStmt {
	var nameExprs []ast.Expr
	for _, result := range funcType.Results.List {
		for _, ident := range result.Names {
			if ident != nil {
				nameExprs = append(nameExprs, ident)
			}
		}
	}
	var sFix = *s
	sFix.Results = nameExprs
	return &sFix
}

func (v *returnsVisitor) NodesVisit(node ast.Node, push bool) bool {
	var (
		funcType *ast.FuncType
		funcName string
	)
	switch s := node.(type) {
	case *ast.FuncDecl:
		// We've found a function
		funcType = s.Type
		funcName = s.Name.Name
	case *ast.FuncLit:
		// We've found a function literal
		funcType = s.Type
		file := v.f.File(s.Pos())
		funcName = fmt.Sprintf("<func():%v>", file.Position(s.Pos()).Line)
	case *ast.ReturnStmt:
		// We've found a possibly naked return statement
		fun := v.functions[len(v.functions)-1]
		funName := nestedFuncName(v.functions)
		if fun.reportNaked && len(s.Results) == 0 && push {
			sFix := nakedReturnFix(s, fun.funcType)
			b := &bytes.Buffer{}
			err := printer.Fprint(b, v.f, sFix)
			if err != nil {
				log.Printf("failed to format named return fix: %s", err)
			}
			v.pass.Report(analysis.Diagnostic{
				Pos:     s.Pos(),
				End:     s.End(),
				Message: fmt.Sprintf("naked return in func `%s` with %d lines of code", funName, fun.funcLength),
				SuggestedFixes: []analysis.SuggestedFix{{
					Message: "explicit return statement",
					TextEdits: []analysis.TextEdit{{
						Pos:     s.Pos(),
						End:     s.End(),
						NewText: b.Bytes()}},
				}},
			})
		}
	}

	if !push {
		if funcType == nil {
			return false
		}
		// Pop function info
		v.functions = v.functions[:len(v.functions)-1]
		return false
	}

	if push && funcType != nil {
		// Push function info to track returns for this function
		file := v.f.File(node.Pos())
		if v.skipTestFiles && strings.HasSuffix(file.Name(), "_test.go") {
			return false
		}
		length := file.Position(node.End()).Line - file.Position(node.Pos()).Line
		if length == 0 {
			// consider functions that finish on the same line as they start as single line functions, not zero lines!
			length = 1
		}
		v.functions = append(v.functions, funcInfo{
			funcType:    funcType,
			funcName:    funcName,
			funcLength:  length,
			reportNaked: uint(length) > v.maxLength && hasNamedReturns(funcType),
		})
	}

	return true
}
