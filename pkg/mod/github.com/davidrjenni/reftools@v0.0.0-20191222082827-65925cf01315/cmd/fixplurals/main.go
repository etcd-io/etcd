// Copyright (c) 2017 David R. Jenni. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Fixplurals removes redundant parameter and result types
// from function signatures.
//
// For example, the following function signature:
//	func fun(a string, b string) (c string, d string)
// becomes:
//	func fun(a, b string) (c, d string)
// after applying fixplurals.
//
// Usage:
//
// 	% fixplurals [-dry] packages
//
// Flags:
//
// -dry: changes are printed to stdout instead of rewriting the source files
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"

	"github.com/kisielk/gotool"
	"golang.org/x/tools/go/loader"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("fixplurals: ")

	dryRun := flag.Bool("dry", false, "dry run: print changes to stdout")
	flag.Parse()

	importPaths := gotool.ImportPaths(flag.Args())
	if len(importPaths) == 0 {
		return
	}

	conf := loader.Config{ParserMode: parser.ParseComments}
	for _, importPath := range importPaths {
		conf.ImportWithTests(importPath)
	}

	prog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}

	for _, pkg := range prog.InitialPackages() {
		for _, file := range pkg.Files {
			filename := conf.Fset.File(file.Pos()).Name()
			ast.Inspect(file, func(node ast.Node) bool {
				if f, ok := node.(*ast.FuncDecl); ok {
					var before []byte
					if *dryRun {
						if before, err = printNode(f.Type, prog.Fset); err != nil {
							log.Fatal(err)
						}
					}
					ch1 := fixPlurals(pkg.Info, f.Type.Params)
					ch2 := fixPlurals(pkg.Info, f.Type.Results)
					if ch1 || ch2 {
						if *dryRun {
							after, err := printNode(f.Type, prog.Fset)
							if err != nil {
								log.Fatal(err)
							}
							fmt.Printf("--- %s\nbefore: %s\nafter:  %s\n\n", filename, string(before), string(after))
						} else {
							src, err := printNode(file, prog.Fset)
							if err != nil {
								log.Fatal(err)
							}
							if err := ioutil.WriteFile(filename, src, 0644); err != nil {
								log.Fatal(err)
							}
						}
					}
				}
				return true
			})
		}
	}
}

func printNode(n ast.Node, fset *token.FileSet) ([]byte, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, n); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fixPlurals(info types.Info, fields *ast.FieldList) (changed bool) {
	if fields == nil || fields.List == nil {
		return
	}

	var prev *ast.Field
	for i := len(fields.List) - 1; i >= 0; i-- {
		field := fields.List[i]
		if i != len(fields.List)-1 && len(field.Names) > 0 && types.Identical(info.Types[prev.Type].Type, info.Types[field.Type].Type) {
			prev.Names = append(field.Names, prev.Names...)
			copy(fields.List[i:], fields.List[i+1:])
			fields.List[len(fields.List)-1] = nil
			fields.List = fields.List[:len(fields.List)-1]
			changed = true
		} else {
			prev = field
		}
	}
	return changed
}
