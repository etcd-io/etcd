// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring that converts between explicitly-typed var
// declarations (var n int = 5) and short assignment statements (n := 5).

package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"reflect"
	"strings"

	"github.com/godoctor/godoctor/text"
	"golang.org/x/tools/go/ast/astutil"
)

// A ToggleVar refactoring converts between explicitly-typed variable
// declarations (var n int = 5) and short assignment statements (n := 5).
type ToggleVar struct {
	RefactoringBase
}

func (r *ToggleVar) Description() *Description {
	return &Description{
		Name:           "Toggle var ⇔ :=",
		Synopsis:       "Toggles between a var declaration and := statement",
		Usage:          "",
		HTMLDoc:        toggleVarDoc,
		Multifile:      false,
		Params:         nil,
		OptionalParams: nil,
		Hidden:         false,
	}
}

func (r *ToggleVar) Run(config *Config) *Result {
	if r.RefactoringBase.Init(config, r.Description()); r.Log.ContainsErrors() {
		return &r.Result
	}

	if r.SelectedNode == nil {
		r.Log.Error("selection cannot be null")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}
	_, nodes, _ := r.Program.PathEnclosingInterval(r.SelectionStart, r.SelectionEnd)
	for i, node := range nodes {
		switch selectedNode := node.(type) {
		case *ast.AssignStmt:
			if selectedNode.Tok == token.DEFINE {
				r.short2var(selectedNode)
				r.UpdateLog(config, true)
			}
			return &r.Result
		case *ast.GenDecl:
			if selectedNode.Tok == token.VAR {
				if _, ok := nodes[i+1].(*ast.File); ok {
					r.Log.Errorf("A global variable cannot be defined using short assign operator.")
					r.Log.AssociateNode(selectedNode)
				} else {
					r.var2short(selectedNode)
					r.UpdateLog(config, true)
				}
				return &r.Result
			}
		}
	}

	r.Log.Errorf("Please select a short assignment (:=) statement or var declaration.\n\nSelected node: %s", reflect.TypeOf(r.SelectedNode))
	r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
	return &r.Result
}

func (r *ToggleVar) short2var(assign *ast.AssignStmt) {
	replacement := r.varDeclString(assign)
	r.Edits[r.Filename].Add(r.Extent(assign), replacement)
	if strings.Contains(replacement, "\n") {
		r.FormatFileInEditor()
	}
}

func (r *ToggleVar) rhsExprs(assign *ast.AssignStmt) []string {
	rhsValue := make([]string, len(assign.Rhs))
	for j, rhs := range assign.Rhs {
		offset, length := r.OffsetLength(rhs)
		rhsValue[j] = string(r.FileContents[offset : offset+length])
	}
	return rhsValue
}

func (r *ToggleVar) varDeclString(assign *ast.AssignStmt) string {
	var buf bytes.Buffer
	replacement := make([]string, len(assign.Rhs))
	path, _ := astutil.PathEnclosingInterval(r.File, assign.Pos(), assign.End())
	for i, rhs := range assign.Rhs {
		switch T := r.SelectedNodePkg.TypeOf(rhs).(type) {
		case *types.Tuple: // function type
			if typeOfFunctionType(T) == "" {
				replacement[i] = fmt.Sprintf("var %s = %s\n",
					r.lhsNames(assign)[i].String(),
					r.rhsExprs(assign)[i])
			} else {
				replacement[i] = fmt.Sprintf("var %s %s = %s\n",
					r.lhsNames(assign)[i].String(),
					typeOfFunctionType(T),
					r.rhsExprs(assign)[i])
			}
		case *types.Named: // package and struct types
			if path[len(path)-1].(*ast.File).Name.Name == T.Obj().Pkg().Name() {
				replacement[i] = fmt.Sprintf("var %s %s = %s\n",
					r.lhsNames(assign)[i].String(),
					T.Obj().Name(),
					r.rhsExprs(assign)[i])
			} else {
				replacement[i] = fmt.Sprintf("var %s %s = %s\n",
					r.lhsNames(assign)[i].String(),
					T,
					r.rhsExprs(assign)[i])
			}
		default:
			replacement[i] = fmt.Sprintf("var %s %s = %s\n",
				r.lhsNames(assign)[i].String(),
				T,
				r.rhsExprs(assign)[i])

		}
		io.WriteString(&buf, replacement[i])
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// typeOfFunctionType receives a type of function's return type, which must be a
// tuple type; if each component has the same type (T, T, T), then it returns
// the type T as a string; otherwise, it returns the empty string.
func typeOfFunctionType(returnType types.Type) string {
	typeArray := make([]string, returnType.(*types.Tuple).Len())
	initialType := returnType.(*types.Tuple).At(0).Type().String()
	finalType := initialType
	for i := 1; i < returnType.(*types.Tuple).Len(); i++ {
		typeArray[i] = returnType.(*types.Tuple).At(i).Type().String()
		if initialType != typeArray[i] {
			finalType = ""
		}
	}
	return finalType
}

func (r *RefactoringBase) lhsNames(assign *ast.AssignStmt) []bytes.Buffer {
	var lhsbuf bytes.Buffer
	buf := make([]bytes.Buffer, len(assign.Lhs))
	for i, lhs := range assign.Lhs {
		offset, length := r.OffsetLength(lhs)
		lhsText := r.FileContents[offset : offset+length]
		if len(assign.Lhs) == len(assign.Rhs) {
			buf[i].Write(lhsText)
		} else {
			lhsbuf.Write(lhsText)
			if i < len(assign.Lhs)-1 {
				lhsbuf.WriteString(", ")
			}
			buf[0] = lhsbuf
		}
	}
	return buf
}

//calls the edit set
func (r *ToggleVar) var2short(decl *ast.GenDecl) {
	start, _ := r.OffsetLength(decl)
	repstrlen := r.Program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Values[0].Pos()).Offset - r.Program.Fset.Position(decl.Pos()).Offset
	r.Edits[r.Filename].Add(&text.Extent{start, repstrlen}, r.shortAssignString(decl))
}

func (r *ToggleVar) varDeclLHS(decl *ast.GenDecl) string {
	offset, _ := r.OffsetLength(decl.Specs[0].(*ast.ValueSpec))
	endOffset := r.Program.Fset.Position(decl.Specs[0].(*ast.ValueSpec).Names[len(decl.Specs[0].(*ast.ValueSpec).Names)-1].End()).Offset
	return string(r.FileContents[offset:endOffset])
}

// returns the shortAssignString string
func (r *ToggleVar) shortAssignString(decl *ast.GenDecl) string {
	return (fmt.Sprintf("%s := ", r.varDeclLHS(decl)))
}

const toggleVarDoc = `
  <h4>Purpose</h4>
  <p>The Toggle var &hArr; := refactoring converts a <tt>var</tt> declaration to a
  short assignment statement (using :=), or vice versa.</p>

  <h4>Usage</h4>
  <ol class="enum">
    <li>Select a local variable declaration (either a <tt>var</tt> declaration
    or a := statement).</li>
    <li>Activate the Toggle var &hArr; := refactoring.</li>
  </ol>

  <p>If a <tt>var</tt> declaration is selected, it will be converted to a short
  assignment statement (:=).  If a short assignment statement is selected, it
  will be converted to a <tt>var</tt> declaration (with an explicit type
  declaration).</p>

  <p>An error or warning will be reported if the selected statements cannot be
  converted.  For example, declarations at the file scope must be declared using
  <tt>var</tt>; they cannot be converted to short assignment statements.</p>

  <h4>Example</h4>
  <p>The example below demonstrates the effect of toggling the highlighted
  declaration of <tt>msg</tt> between a short assignment statement and a
  <tt>var</tt> declaration.</p>
  <table cellspacing="5" cellpadding="15" style="border: 0;">
    <tr>
      <th>Before</th><th>&nbsp;</th><th>After</th>
    </tr>
    <tr>
      <td class="dotted">
        <pre>package main
import "fmt"

func main() {
    <span class="highlight">msg := hello</span>
    fmt.Println(msg)
}</pre>
      </td>
      <td>&nbsp;&nbsp;&nbsp;&nbsp;&hArr;&nbsp&nbsp;&nbsp;&nbsp;</td>
      <td class="dotted">
        <pre>package main
import "fmt"

func main() {
    <span class="highlight">var msg string = hello</span>
    fmt.Println(msg)
}</pre>
      </td>
    </tr>
  </table>
`
