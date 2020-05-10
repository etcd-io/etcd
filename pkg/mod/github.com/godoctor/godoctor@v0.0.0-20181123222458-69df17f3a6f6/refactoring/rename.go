// Copyright 2015-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a refactoring to rename variables, functions, methods,
// types, interfaces, and packages.

package refactoring

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/tools/go/loader"

	"github.com/godoctor/godoctor/analysis/names"
	"github.com/godoctor/godoctor/text"
)

// Rename is a refactoring that changes the names of variables, functions,
// methods, types, interfaces, and packages in Go programs.  It attempts to
// prevent name changes that will introduce syntactic or semantic errors into
// the Program.
type Rename struct {
	RefactoringBase
	newName string // New name to be given to the selected identifier
}

func (r *Rename) Description() *Description {
	return &Description{
		Name:      "Rename",
		Synopsis:  "Changes the name of an identifier",
		Usage:     "<new_name>",
		HTMLDoc:   renameDoc,
		Multifile: true,
		Params: []Parameter{{
			Label:        "New Name:",
			Prompt:       "What to rename this identifier to.",
			DefaultValue: "",
		}},
		OptionalParams: nil,
		Hidden:         false,
	}
}

func (r *Rename) Run(config *Config) *Result {
	r.Init(config, r.Description())
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}

	if r.SelectedNode == nil {
		r.Log.Error("Please select an identifier to rename.")
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}

	r.newName = config.Args[0].(string)
	if r.newName == "" {
		r.Log.Error("newName cannot be empty")
		return &r.Result
	}
	if !isIdentifierValid(r.newName) {
		r.Log.Errorf("The new name \"%s\" is not a valid Go identifier", r.newName)
		return &r.Result
	}
	if isReservedWord(r.newName) {
		r.Log.Errorf("The new name \"%s\" is a reserved word", r.newName)
		return &r.Result
	}

	// If no ident was found, try to get hold of the concrete node type and get it's name
	var ident *ast.Ident
	switch node := r.SelectedNode.(type) {
	case *ast.FuncDecl:
		ident = node.Name
	case *ast.Ident:
		ident = node
	default:
		r.Log.Errorf("Please select an identifier to rename. "+
			"(Selected node: %s)", reflect.TypeOf(r.SelectedNode))
		r.Log.AssociatePos(r.SelectionStart, r.SelectionEnd)
		return &r.Result
	}

	// FIXME: Check if main function (not type/var/etc.) -JO
	if ident.Name == "main" && r.SelectedNodePkg.Pkg.Name() == "main" {
		r.Log.Error("The \"main\" function in the \"main\" package cannot be renamed: it will eliminate the program entrypoint")
		r.Log.AssociateNode(ident)
		return &r.Result
	}

	if isPredeclaredIdentifier(ident.Name) {
		r.Log.Errorf("selected predeclared  identifier \"%s\" , it cannot be renamed", ident.Name)
		r.Log.AssociateNode(ident)
		return &r.Result
	}

	if ast.IsExported(ident.Name) && !ast.IsExported(r.newName) {
		r.Log.Warn("Renaming an exported name to an unexported name will introduce errors outside the package in which it is declared.")
	}

	r.rename(ident, r.SelectedNodePkg)
	r.UpdateLog(config, false)
	return &r.Result

}

func isIdentifierValid(newName string) bool {
	b, _ := regexp.MatchString("^[\\p{L}|_][\\p{L}|_|\\p{N}]*$", newName)
	return b
}

func isPredeclaredIdentifier(selectedIdentifier string) bool {
	b, _ := regexp.MatchString("^(bool|byte|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr|true|false|iota|nil|append|cap|close|complex|copy|delete|imag|len|make|new|panic|print|println|real|recover)$", selectedIdentifier)
	return b
}

func isReservedWord(newName string) bool {
	b, _ := regexp.MatchString("^(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)$", newName)
	return b
}

func (r *Rename) rename(ident *ast.Ident, pkgInfo *loader.PackageInfo) {
	obj := pkgInfo.ObjectOf(ident)

	if obj == nil && r.selectedTypeSwitchVar(ident) == nil {
		r.Log.Errorf("The selected identifier cannot be " +
			"renamed.  (Package and cgo renaming are not " +
			"currently supported.)")
		r.Log.AssociateNode(ident)
		return
	}

	if obj != nil && isInGoRoot(r.Program.Fset.Position(obj.Pos()).Filename) {
		r.Log.Errorf("%s is defined in $GOROOT and cannot be renamed",
			ident.Name)
		r.Log.AssociateNode(ident)
		return
	}
	if conflict := names.FindConflict(obj, r.newName); conflict != nil {
		r.Log.Errorf("Renaming %s to %s may cause conflicts with an existing declaration", ident.Name, r.newName)
		r.Log.AssociatePos(conflict.Pos(), conflict.Pos())
	}
	var scope *types.Scope
	var idents map[*ast.Ident]bool
	if ts := r.selectedTypeSwitchVar(ident); ts != nil {
		scope = types.NewScope(nil, ts.Pos(), ts.End(), "artificial scope for typeswitch")
		idents = names.FindTypeSwitchVarOccurrences(ts, r.SelectedNodePkg, r.Program)
	} else {
		if obj != nil {
			scope = obj.Parent()
		}
		idents = names.FindOccurrences(obj, r.Program)
	}

	r.addOccurrences(ident.Name, scope, r.extents(idents, r.Program.Fset))
}

func (r *Rename) selectedTypeSwitchVar(ident *ast.Ident) *ast.TypeSwitchStmt {
	obj := r.SelectedNodePkg.ObjectOf(ident)

	for _, n := range r.PathEnclosingSelection {
		if typeSwitch, ok := n.(*ast.TypeSwitchStmt); ok {
			if asgt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
				if len(asgt.Lhs) == 1 &&
					asgt.Tok == token.DEFINE &&
					asgt.Lhs[0] == r.SelectedNode {
					return typeSwitch
				}
			}
			for _, stmt := range typeSwitch.Body.List {
				cc := stmt.(*ast.CaseClause)
				if r.SelectedNodePkg.Implicits[cc] == obj {
					return typeSwitch
				}
			}
		}
	}
	return nil
}

func (r *Rename) extents(ids map[*ast.Ident]bool, fset *token.FileSet) map[string][]*text.Extent {
	result := map[string][]*text.Extent{}
	for id := range ids {
		pos := fset.Position(id.Pos())
		if _, ok := result[pos.Filename]; !ok {
			result[pos.Filename] = []*text.Extent{}
		}
		result[pos.Filename] = append(result[pos.Filename],
			&text.Extent{Offset: pos.Offset, Length: len(id.Name)})
	}

	sorted := map[string][]*text.Extent{}
	for fname, extents := range result {
		sorted[fname] = text.Sort(extents)
	}
	return sorted
}

func (r *Rename) addOccurrences(name string, scope *types.Scope, allOccurrences map[string][]*text.Extent) {
	hasOccsInGoRoot := false
	for filename, occurrences := range allOccurrences {
		if isInGoRoot(filename) {
			hasOccsInGoRoot = true
		} else {
			if r.Edits[filename] == nil {
				r.Edits[filename] = text.NewEditSet()
			}
			for _, occurrence := range occurrences {
				r.Edits[filename].Add(occurrence, r.newName)
			}
			_, file := r.fileNamed(filename)
			commentOccurrences := names.FindInComments(
				name, file, scope, r.Program.Fset)
			for _, occurrence := range commentOccurrences {
				r.Edits[filename].Add(occurrence, r.newName)
			}
		}
	}
	if hasOccsInGoRoot {
		r.Log.Warnf("Occurrences were found in files under $GOROOT, but these will not be renamed")
	}
}

func isInGoRoot(absPath string) bool {
	goRoot := os.Getenv("GOROOT")
	if goRoot == "" {
		goRoot = runtime.GOROOT()
	}

	if !strings.HasSuffix(goRoot, string(filepath.Separator)) {
		goRoot += string(filepath.Separator)
	}
	return strings.HasPrefix(absPath, goRoot)
}

func (r *Rename) fileNamed(filename string) (*loader.PackageInfo, *ast.File) {
	absFilename, _ := filepath.Abs(filename)
	for _, pkgInfo := range r.Program.AllPackages {
		for _, f := range pkgInfo.Files {
			thisFile := r.Program.Fset.Position(f.Pos()).Filename
			if thisFile == filename || thisFile == absFilename {
				return pkgInfo, f
			}
		}
	}
	return nil, nil
}

const renameDoc = `
  <h4>Purpose</h4>
  <p>The Rename refactoring is used to change the names of variables,
  functions, methods, and types.  Package renaming is not currently
  supported.</p>

  <h4>Usage</h4>
  <ol class="enum">
    <li>Select an identifier to be renamed.</li>
    <li>Activate the Rename refactoring.</li>
    <li>Enter a new name for the identifier.</li>
  </ol>

  <p>An error or warning will be reported if:</p>
  <ul>
    <li>The renaming could introduce errors (e.g., two functions would have the
    same name).</li>
    <li>The necessary changes cannot be made (e.g., the renaming would change 
    the name of a function in the Go standard library).</li>
  </ul>

  <h4>Example</h4>
  <p>The example below demonstrates the effect of renaming the highlighted
  occurrence of <tt>hello</tt> to <tt>goodnight</tt>.  Note that there are two
  different variables named <tt>hello</tt>; since the local identifier was
  selected, only references to that variable are renamed, as shown.</p>
  <table cellspacing="5" cellpadding="15" style="border: 0;">
    <tr>
      <th>Before</th><th>&nbsp;</th><th>After</th>
    </tr>
    <tr>
      <td class="dotted">
        <pre>package main
import "fmt"

var hello = ":-("

func main() {
    hello = ":-)"
    var hello string = "hello"
    var world string = "world"
    hello = <span class="highlight">hello</span> + ", " + world
    hello += "!"
    fmt.Println(hello)
}</pre>
      </td>
      <td>&nbsp;&nbsp;&nbsp;&nbsp;&rArr;&nbsp&nbsp;&nbsp;&nbsp;</td>
      <td class="dotted">
        <pre>package main
import "fmt"

var hello = ":-("

func main() {
    hello = ":-)"
    var <span class="highlight">goodnight</span> string = "hello"
    var world string = "world"
    <span class="highlight">goodnight</span> = <span class="highlight">goodnight</span> + ", " + world
    <span class="highlight">goodnight</span> += "!"
    fmt.Println(goodnight)
}</pre>
      </td>
    </tr>
  </table>

  <h4>Limitations</h4>
  <ul>
    <li><b>Package renaming is not currently supported.</b>  Package renaming
    requires renaming directories, which causes files to move on the file
    system.  When the refactoring is activated from a text editor (e.g., Vim),
    the editor needs to be notified of such changes and respond appropriately.
    Additional work is needed to support this behavior.</li>
    <li><b>Name collision detection is overly conservative.</b>  If renaming
    will introduce shadowing, this is reported as an error, even if it will not
    change the program's semantics.</li>
  </ul>
`
