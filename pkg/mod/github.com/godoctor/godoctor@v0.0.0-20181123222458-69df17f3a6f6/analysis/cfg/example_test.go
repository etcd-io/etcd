// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cfg_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/godoctor/godoctor/analysis/cfg"
)

func ExampleCFG() {
	src := `
    package main

    import "fmt"

    func main() {
      for {
        if 1 > 0 {
          fmt.Println("my computer works")
        } else {
          fmt.Println("something has gone terribly wrong")
        }
      }
    }
  `

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	funcOne := f.Decls[1].(*ast.FuncDecl)
	c := cfg.FromFunc(funcOne)
	_ = c.Blocks() // for 100% coverage ;)

	ast.Inspect(f, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.IfStmt:
			s := c.Succs(stmt)
			p := c.Preds(stmt)

			fmt.Println(len(s))
			fmt.Println(len(p))
		}
		return true
	})
	// Output:
	// 2
	// 1
}
