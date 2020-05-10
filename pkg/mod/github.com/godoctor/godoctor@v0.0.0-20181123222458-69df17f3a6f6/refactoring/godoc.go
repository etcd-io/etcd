// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE File.

// This File defines a refactoring that adds GoDoc comments to all exported
// top-level declarations in a File.

package refactoring

import (
	"go/ast"
	"go/token"

	"github.com/godoctor/godoctor/text"
)

// The AddGoDoc refactoring adds GoDoc comments to all exported top-level
// declarations in a File.
type AddGoDoc struct {
	RefactoringBase
}

func (r *AddGoDoc) Description() *Description {
	return &Description{
		Name:           "Add GoDoc",
		Synopsis:       "Adds stub GoDoc comments where they are missing",
		Usage:          "",
		HTMLDoc:        godocDoc,
		Multifile:      false,
		Params:         nil,
		OptionalParams: nil,
		Hidden:         false,
	}
}

func (r *AddGoDoc) Run(config *Config) *Result {
	r.Init(config, r.Description())
	r.Log.ChangeInitialErrorsToWarnings()
	if r.Log.ContainsErrors() {
		return &r.Result
	}

	r.removeSemicolons()
	r.addComments()
	r.FormatFileInEditor()
	return &r.Result
}

// removeSemicolons iterates through the top-level declarations in a File and
// the specs of general declarations, and if two consecutive declarations occur
// on the same line, splits them onto separate lines.  The intention is to
// split semicolon-separated declarations onto separate lines.
func (r *AddGoDoc) removeSemicolons() {
	for i, d := range r.File.Decls {
		if i > 0 {
			r.removeSemicolonBetween(r.File.Decls[i-1], r.File.Decls[i], "\n\n")
		}
		if decl, ok := d.(*ast.GenDecl); ok {
			for j, spec := range decl.Specs {
				if spec, ok := spec.(*ast.TypeSpec); ok {
					if ast.IsExported(spec.Name.Name) && spec.Doc == nil && j > 0 {
						r.removeSemicolonBetween(decl.Specs[j-1], decl.Specs[j], "\n")
					}
				}
			}
		}

	}
}

func (r *AddGoDoc) removeSemicolonBetween(node1, node2 ast.Node, replacement string) {
	// Check if the 2 declarations are on the same line
	line1 := r.Program.Fset.Position(node1.Pos()).Line
	line2 := r.Program.Fset.Position(node2.Pos()).Line
	if line1 == line2 {
		// Replace text between the end of the first declaration and
		// the start of the second declaration with the given
		// separators.  If there are comments, they will be eliminated,
		// but this should occur rarely enough we'll ignore it for now.
		offset := r.Program.Fset.Position(node1.End()).Offset
		length := r.Program.Fset.Position(node2.Pos()).Offset - offset
		r.Edits[r.Filename].Add(&text.Extent{offset, length}, replacement)
	}
}

// addComments inserts a comment immediately before all exported top-level
// declarations that do not already have an associated doc comment
func (r *AddGoDoc) addComments() {
	for _, d := range r.File.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl: // function or method declaration
			if ast.IsExported(decl.Name.Name) && decl.Doc == nil {
				r.addComment(decl, decl.Name.Name) //, 1)
			}
		case *ast.GenDecl: // type and value declarations
			switch decl.Tok {
			case token.IMPORT:
				continue
			default: // CONST, TYPE, or VAR
				r.addCommentToGenDecl(decl)
			}
		}
	}
}

// addComment inserts the given comment string immediately before the given
// declaration
func (r *AddGoDoc) addComment(decl ast.Node, comment string) {
	comment = "// " + comment + " TODO: NEEDS COMMENT INFO\n"
	insertOffset := r.Program.Fset.Position(decl.Pos()).Offset
	r.Edits[r.Filename].Add(&text.Extent{insertOffset, 0}, comment)
}

// addCommentToGenDecl adds doc comments to a GenDecl (var, type, or const).
//
// A GenDecl can have a doc comment of its own, or if it contains several
// declarations, each one can have its own doc comment.  If the user has
// already commented at least one individual declaration, we comment the rest
// to be consistent with their style; if not, we add a comment for the GenDecl
// as a whole, to avoid being too obtrusive.
func (r *AddGoDoc) addCommentToGenDecl(decl *ast.GenDecl) {
	if decl.Doc != nil {
		return
	}

	if decl.Lparen.IsValid() {
		// Multiple declarations
		commentIndividualSpecs, s := r.collectSpecsWithoutDoc(decl)
		if commentIndividualSpecs {
			for name, spec := range s {
				r.addComment(spec, name)
			}
		} else {
			r.addComment(decl, "")
		}
	} else {
		// Only one declaration
		name := getName(decl.Specs[0])
		if ast.IsExported(name) && decl.Doc == nil {
			r.addComment(decl, name)
		}
	}
}

// collectSpecsWithoutDoc returns (1) a Boolean value indicating whether at
// least one spec has a doc comment, and (2) a map from names to ast.Spec nodes
// indicating those specs that do not have doc comments
func (r *AddGoDoc) collectSpecsWithoutDoc(decl *ast.GenDecl) (bool, map[string]ast.Spec) {
	commentIndividualSpecs := false
	specs := make(map[string]ast.Spec)
	for _, spec := range decl.Specs {
		name := getName(spec)
		if ast.IsExported(name) {
			if !hasDoc(spec) {
				specs[name] = spec
			} else {
				// They're commenting individual specs; we should too
				commentIndividualSpecs = true
			}
		}
	}
	return commentIndividualSpecs, specs
}

func getName(spec ast.Spec) string {
	switch s := spec.(type) {
	case *ast.ValueSpec:
		return s.Names[0].Name
	case *ast.TypeSpec:
		return s.Name.Name
	default:
		panic("Unexpected spec type")
	}
}

func hasDoc(spec ast.Spec) bool {
	switch s := spec.(type) {
	case *ast.ValueSpec:
		return s.Doc != nil
	case *ast.TypeSpec:
		return s.Doc != nil
	default:
		panic("Unexpected spec type")
	}
}

const godocDoc = `
  <h4>Purpose</h4>
  <p>This refactoring searches a file for exported declarations that do not have
  GoDoc comments and adds TODO comment stubs to those declarations.</p>
  <p>The refactored source code is formatted (similarly to gofmt).</p>

  <h4>Usage</h4>
  <p>This refactoring is applied to an entire file.  It does not require any
  particular text to be selected, and it does not prompt for any additional user
  input.</p>

  <h4>Example</h4>
  <p>In the following example, Exported, Shaper, and Rectangle are all exported
  but lack doc comments.  This refactoring adds a TODO comment for each.</p>
  <table cellspacing="5" cellpadding="15" style="border: 0;">
    <tr>
      <th>Before</th><th>&nbsp;</th><th>After</th>
    </tr>
    <tr>
      <td class="dotted">
  <pre>package main
import "fmt"

func main() {
    Exported()
}

func Exported() {
    fmt.Println("Hello, Go")
}

type Shaper interface {
}
 
type Rectangle struct {
}
  </pre>
      </td>
      <td>&nbsp;&nbsp;&nbsp;&nbsp;&rArr;&nbsp;&nbsp;&nbsp;&nbsp;</td>
      <td class="dotted">
      <pre>package main
import "fmt"

func main() {
    Exported()
}

<span class="highlight">// Exported TODO: NEEDS COMMENT INFO</span>
func Exported() {
    fmt.Println("Hello, Go")
}

<span class="highlight">// Exported TODO: NEEDS COMMENT INFO</span>
type Shaper interface {
}
  
<span class="highlight">// Exported TODO: NEEDS COMMENT INFO</span>
type Rectangle struct {
}
</pre>
      </td>
    </tr>
  </table>

  <h4>Limitations</h4>
  <ul>
    <li>When a single declaration contains non-exported declarations
        followed by exported declarations (e.g.,
        <tt>var x, Y int</tt>), a documentation comment will not be
        added.</li>
  </ul>
`
