// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dataflow_test

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/loader"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/godoctor/godoctor/analysis/dataflow"
)

func ExampleReachingDefs() {
	src := `
    package main

    import "fmt"

    func main() {
      a := 1
      b := 2
      c := 3
      a := b
      a, b := b, a
      c := a + b
    }
  `

	// use own loader config, this is just necessary
	var config loader.Config
	f, err := config.ParseFile("testing", src)
	if err != nil {
		return // probably don't proceed
	}
	config.CreateFromFiles("testing", f)
	prog, err := config.Load()
	if err != nil {
		return
	}

	funcOne := f.Decls[1].(*ast.FuncDecl)
	c := cfg.FromFunc(funcOne)
	du := dataflow.DefUse(c, prog.Created[0])

	ast.Inspect(f, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case ast.Stmt:
			fmt.Println(len(du[stmt]))
			// do as you please
		}
		return true
	})
}

func ExampleLiveVars() {
	src := `
    package main

    import "fmt"

    func main() {
      a := 1
      b := 2
      c := 3
      a := b
      a, b := b, a
      c := a + b
    }
  `

	// use own loader config, this is just necessary
	var config loader.Config
	f, err := config.ParseFile("testing", src)
	if err != nil {
		return // probably don't proceed
	}
	config.CreateFromFiles("testing", f)
	prog, err := config.Load()
	if err != nil {
		return
	}

	funcOne := f.Decls[1].(*ast.FuncDecl)
	cfg := cfg.FromFunc(funcOne)
	in, out := dataflow.LiveVars(cfg, prog.Created[0])

	ast.Inspect(f, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case ast.Stmt:
			_, _ = in[stmt], out[stmt]
			// do as you please
		}
		return true
	})
}
