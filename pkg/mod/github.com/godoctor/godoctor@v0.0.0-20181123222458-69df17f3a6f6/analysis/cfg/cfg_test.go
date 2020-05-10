// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cfg

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"testing"
)

const (
	START = 0
	END   = 100000000 // if there's this many statements, may god have mercy on your soul
)

func TestBlockStmt(t *testing.T) {
	c := getWrapper(t, `
package main

func foo(i int) {
  {
    {
      bar(i) //1
    }
  }
}
func bar(i int) {}`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, END)
}

func TestIfElseIfGoto(t *testing.T) {
	c := getWrapper(t, `
  package main

  func main() {
    i := 5              //1
    i++                 //2
    if i == 6 {         //3
        goto ABC        //4
    } else if i == 8 {  //5
        goto DEF        //6
    }
  ABC: fmt.Println("6") //7, 8
  DEF: fmt.Println("8") //9, 10
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3)
	c.expectSuccs(t, 3, 4, 5)
	c.expectSuccs(t, 4, 7)
	c.expectSuccs(t, 5, 6, 7)
	c.expectSuccs(t, 6, 9)
	c.expectSuccs(t, 7, 8)
	c.expectSuccs(t, 8, 9)
	c.expectSuccs(t, 9, 10)
}

func TestDoubleForBreak(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    for { //1
      for { //2
        break //3
      }
    }
    print("this") //4
    //END
  }`)

	//            t, stmt, ...successors
	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 4)
	c.expectSuccs(t, 2, 3, 1)
	c.expectSuccs(t, 3, 1)

	c.expectPreds(t, 3, 2)
	c.expectPreds(t, 4, 1)
	c.expectPreds(t, END, 4)
}

func TestFor(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    for i := 0; i < c; i++ { // 2, 1, 3
      println(i) //4
    }
    println(c) //5
    //END
  }`)

	c.expectSuccs(t, START, 2)
	c.expectSuccs(t, 2, 1)
	c.expectSuccs(t, 1, 4, 5)
	c.expectSuccs(t, 4, 3)
	c.expectSuccs(t, 3, 1)

	c.expectPreds(t, 5, 1)
	c.expectPreds(t, END, 5)
}

func TestForContinue(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    for i := 0; i < c; i++ { // 2, 1, 3
      println(i) // 4
      if i > 1 { // 5
        continue // 6
      } else {
        break    // 7
      }
    }
    println(c) // 8
    //END
  }`)

	c.expectSuccs(t, START, 2)
	c.expectSuccs(t, 2, 1)
	c.expectSuccs(t, 1, 4, 8)
	c.expectSuccs(t, 6, 3)
	c.expectSuccs(t, 3, 1)
	c.expectSuccs(t, 7, 8)

	c.expectPreds(t, END, 8)
}

func TestIfElse(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    if c := 1; c > 0 { // 2, 1
      print("there") // 3
    } else {
      print("nowhere") // 4
    }
    //END
  }`)

	c.expectSuccs(t, START, 2)
	c.expectSuccs(t, 2, 1)
	c.expectSuccs(t, 1, 3, 4)

	c.expectPreds(t, 4, 1)
	c.expectPreds(t, END, 4, 3)
}

func TestIfNoElse(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    if c > 0 && true { // 1
      println("here") // 2
    }
    print("there") // 3
    //END
  }
  `)
	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 3)

	c.expectPreds(t, 3, 1, 2)
	c.expectPreds(t, END, 3)
}

func TestIfElseIf(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    if c > 0 { //1
      println("here") //2
    } else if c == 0 { //3
      println("there") //4
    } else {
      println("everywhere") //5
    }
    print("almost end") //6
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 3)
	c.expectSuccs(t, 2, 6)
	c.expectSuccs(t, 3, 4, 5)
	c.expectSuccs(t, 4, 6)
	c.expectSuccs(t, 5, 6)

	c.expectPreds(t, 6, 5, 4, 2)
}

func TestDefer(t *testing.T) {
	c := getWrapper(t, `
package main

func foo() {
  //START
  print("this") //1
  defer print("one") //2
  if 1 != 0 { //3
    defer print("two") //4
    return //5
  }
  print("that") //6
  defer print("three") //7
  return //8
  //END
}
`)
	c.expectSuccs(t, 3, 5, 6)
	c.expectSuccs(t, 5, END)

	c.expectPreds(t, 8, 6)
	c.expectDefers(t, 2, 4, 7)
}

func TestRange(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo() { 
    //START
    c := []int{1, 2, 3} //1
  lbl: //2
    for i, v := range c { //3
      for j, k := range c { //4
        if i == j { //5
          break //6
        }
        print(i*i) //7
        break lbl //8
      }
    }
    //END
  }
  `)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3)
	c.expectSuccs(t, 3, 4, END)
	c.expectSuccs(t, 4, 5, 3)
	c.expectSuccs(t, 6, 3)
	c.expectSuccs(t, 8, END)

	c.expectPreds(t, END, 8, 3)
}

func TestTypeSwitchDefault(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(s ast.Stmt) {
    //START
    switch s.(type) { // 1, 2
    case *ast.AssignStmt: //3
      print("assign") //4
    case *ast.ForStmt: //5
      print("for") //6
    default: //7
      print("default") //8
    }
    //END
  }
  `)

	c.expectSuccs(t, 2, 3, 5, 7)

	c.expectPreds(t, END, 8, 6, 4)
}

func TestTypeSwitchNoDefault(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(s ast.Stmt) {
  //START
  switch x := 1; s := s.(type) { // 2, 1, 3
  case *ast.AssignStmt: // 4
    print("assign") // 5
  case *ast.ForStmt: // 6
    print("for") // 7
  }
  //END
  }
`)

	c.expectSuccs(t, START, 2)
	c.expectSuccs(t, 2, 1)
	c.expectSuccs(t, 1, 3)
	c.expectSuccs(t, 3, 4, 6, END)
}

func TestSwitch(t *testing.T) {
	c := getWrapper(t, `
  package main
  
  func foo(c int) {
    //START
    print("hi") //1
    switch c+=1; c { //2, 3
    case 1: //4
      print("one") //5
      fallthrough //6
    case 2: //7
      break //8
      print("two") //9
    case 3: //10
    case 4: //11
      if i > 3 { //12
        print("> 3") //13
      } else { 
        print("< 3") //14
      }
    default: //15
      print("done") //16
    }
    //END
  }
  `)
	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 3)
	c.expectSuccs(t, 3, 2)
	c.expectSuccs(t, 2, 4, 7, 10, 11, 15)

	c.expectPreds(t, END, 16, 14, 13, 10, 9, 8)
}

func TestLabeledFallthrough(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    switch c { //1
    case 1: //2
      print("one") //3
      goto lbl //4
    case 2: //5
      print("two") //6
    lbl: //7
      mlbl: //8
        fallthrough //9
    default: //10
      print("number") //11
    }
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2, 5, 10)
	c.expectSuccs(t, 4, 7)
	c.expectSuccs(t, 7, 8)
	c.expectSuccs(t, 8, 9)
	c.expectSuccs(t, 9, 11)
	c.expectSuccs(t, 10, 11)

	c.expectPreds(t, END, 11)
}

func TestSelectDefault(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    //START
    ch := make(chan int) // 1

    // go func() { // 2
      // for i := 0; i < c; i++ { // 4, 3, 5
        // ch <- c // 6
      // }
    // }()

    select { // 2
    case got := <- ch: // 3, 4
      print(got) // 5
    default: // 6
      print("done") // 7
    }
    //END
  }`)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3, 6)
	c.expectSuccs(t, 3, 4)
	c.expectSuccs(t, 4, 5)

	c.expectPreds(t, END, 5, 7)
}

// TODO modify ast.Inspect for go statements
// TODO also, does a go statement have control ever?
//func TestClosure(t *testing.T) {
//c := getWrapper(t, `
//package main

//func foo(c int) {
////START
//if c > 0 { //1
//go func(i int) { //2
//println(i)
//}(c)
//}
//println(c) //3
////END
//}`)

//c.printAST()

//c.expectSuccs(t, START, 1)
//c.expectSuccs(t, 1, 2, 3)
//}

func TestDietyExistence(t *testing.T) {
	c := getWrapper(t, `
  package main

  func foo(c int) {
    b := 7 // 1
  hello: // 2
    for c < b { // 3
      for { // 4
        if c&2 == 2 { // 5
          continue hello // 6
          println("even") // 7
        } else if c&1 == 1 { // 8
          defer println("sup") // 9
          println("odd") // 10
          break // 11
        } else {
          println("something wrong") // 12
          goto ending // 13
        }
        println("something") // 14
      }
      println("poo") // 15
    }
    println("hello") // 16
    ending: // 17
  }
  `)

	c.expectSuccs(t, START, 1)
	c.expectSuccs(t, 1, 2)
	c.expectSuccs(t, 2, 3)
	c.expectSuccs(t, 3, 4, 16)
	c.expectSuccs(t, 4, 5, 15)
	c.expectSuccs(t, 5, 6, 8)
	c.expectSuccs(t, 6, 3)
	c.expectSuccs(t, 7, 14)
	c.expectSuccs(t, 8, 10, 12)

	c.expectDefers(t, 9)

	c.expectSuccs(t, 10, 11)
	c.expectSuccs(t, 11, 15)
	c.expectSuccs(t, 12, 13)
	c.expectSuccs(t, 13, 17)
	c.expectSuccs(t, 14, 4)
	c.expectSuccs(t, 15, 3)
	c.expectSuccs(t, 16, 17)
}

// lo and behold how it's done -- caution: disgust may ensue
type CFGWrapper struct {
	cfg   *CFG
	exp   map[int]ast.Stmt
	stmts map[ast.Stmt]int
	objs  map[string]*ast.Object
	fset  *token.FileSet
	f     *ast.File
}

// uses first function in given string to produce CFG
// w/ some other convenient fields for printing in test
// cases when need be...
func getWrapper(t *testing.T, str string) *CFGWrapper {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", str, 0)
	if err != nil {
		t.Error(err.Error())
		t.FailNow()
		return nil
	}
	cfg := FromFunc(f.Decls[0].(*ast.FuncDecl)) //yes, so all test cases take first function
	v := make(map[int]ast.Stmt)
	stmts := make(map[ast.Stmt]int)
	objs := make(map[string]*ast.Object)
	i := 1
	ast.Inspect(f.Decls[0].(*ast.FuncDecl), func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			objs[x.Name] = x.Obj
		case ast.Stmt:
			switch x.(type) {
			case *ast.BlockStmt:
				return true
			}
			v[i] = x
			stmts[x] = i
			i++
			//TODO skip over any statements w/i inner func... as our graph does
		}
		return true
	})
	v[END] = cfg.Exit
	v[START] = cfg.Entry
	if len(v) != len(cfg.blocks)+len(cfg.Defers) {
		t.Logf("expected %d vertices, got %d --construction error", len(v), len(cfg.blocks))
	}
	return &CFGWrapper{cfg, v, stmts, objs, fset, f}
}

func (c *CFGWrapper) expIntsToStmts(args []int) map[ast.Stmt]struct{} {
	stmts := make(map[ast.Stmt]struct{})
	for _, a := range args {
		stmts[c.exp[a]] = struct{}{}
	}
	return stmts
}

// give generics
func expectFromMaps(actual, exp map[ast.Stmt]struct{}) (dnf, found map[ast.Stmt]struct{}) {
	for stmt := range exp {
		if _, ok := actual[stmt]; ok {
			delete(exp, stmt)
			delete(actual, stmt)
		}
	}

	return exp, actual
}

func (c *CFGWrapper) expectDefers(t *testing.T, exp ...int) {
	actualDefers := make(map[ast.Stmt]struct{})
	for _, d := range c.cfg.Defers {
		actualDefers[d] = struct{}{}
	}

	expDefers := c.expIntsToStmts(exp)
	dnf, found := expectFromMaps(actualDefers, expDefers)

	for stmt := range dnf {
		t.Error("did not find", c.stmts[stmt], "in defers")
	}

	for stmt := range found {
		t.Error("found", c.stmts[stmt], "as a defer")
	}
}

func (c *CFGWrapper) expectSuccs(t *testing.T, s int, exp ...int) {
	if _, ok := c.cfg.blocks[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
		return
	}

	//get successors for stmt s as slice, put in map
	actualSuccs := make(map[ast.Stmt]struct{})
	for _, v := range c.cfg.Succs(c.exp[s]) {
		actualSuccs[v] = struct{}{}
	}

	expSuccs := c.expIntsToStmts(exp)
	dnf, found := expectFromMaps(actualSuccs, expSuccs)

	for stmt := range dnf {
		t.Error("did not find", c.stmts[stmt], "in successors for", s)
	}

	for stmt := range found {
		t.Error("found", c.stmts[stmt], "as a successor for", s)
	}
}

func (c *CFGWrapper) expectPreds(t *testing.T, s int, exp ...int) {
	if _, ok := c.cfg.blocks[c.exp[s]]; !ok {
		t.Error("did not find parent", s)
	}

	//get predecessors for stmt s as slice, put in map
	actualPreds := make(map[ast.Stmt]struct{})
	for _, v := range c.cfg.Preds(c.exp[s]) {
		actualPreds[v] = struct{}{}
	}

	expPreds := c.expIntsToStmts(exp)
	dnf, found := expectFromMaps(actualPreds, expPreds)

	for stmt := range dnf {
		t.Error("did not find", c.stmts[stmt], "in predecessors for", s)
	}

	for stmt := range found {
		t.Error("found", c.stmts[stmt], "as a predecessor for", s)
	}
}

//prints given AST
func (c *CFGWrapper) printAST() {
	ast.Print(c.fset, c.f)
}

func (c *CFGWrapper) printDOT() {
	f, err := os.Create("graph.dot")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	c.cfg.PrintDot(f, c.fset, func(ast.Stmt) string { return "" })
}

func TestPrintDot(t *testing.T) {
	c := getWrapper(t, `
  package main

  func main() {
    i := 5              //1
    i++                 //2
  }`)

	var buf bytes.Buffer
	c.cfg.PrintDot(&buf, c.fset, func(s ast.Stmt) string {
		if _, ok := s.(*ast.AssignStmt); ok {
			return "!"
		} else {
			return ""
		}
	})
	dot := buf.String()

	expected := []string{
		`^digraph mgraph {
mode="heir";
splines="ortho";

`,
		"\"assignment - line 5\\\\n!\" -> \"increment statement - line 6\"\n",
		"\"ENTRY\" -> \"assignment - line 5\\\\n!\"\n",
		"\"increment statement - line 6\" -> \"EXIT\"\n",
	}
	// The order of the three lines may vary (they're from a map), so
	// just make sure all three lines appear somewhere
	for _, re := range expected {
		ok, _ := regexp.MatchString(re, dot)
		if !ok {
			t.Fatalf("[%s]", dot)
		}
	}
}
