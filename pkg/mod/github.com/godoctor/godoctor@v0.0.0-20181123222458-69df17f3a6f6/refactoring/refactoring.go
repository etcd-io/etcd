// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE File.

// This File defines the Refactoring interface, the RefactoringBase struct, and
// several methods common to refactorings based on RefactoringBase, including
// a base implementation of the Run method.

// Package refactoring contains all of the refactorings supported by the Go
// Doctor, as well as types (such as refactoring.Log) used to interface with
// those refactorings.
package refactoring

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/godoctor/godoctor/filesystem"
	"github.com/godoctor/godoctor/text"
	"golang.org/x/tools/go/loader"
)

// The maximum number of errors from the go/loader that will be reported
const maxInitialErrors = 10

// Description of a parameter for a refactoring.
//
// Some refactorings require additional input from the user besides a text
// selection.  For example, in a Rename refactoring, the user may select an
// identifier to rename, but the refactoring tool must also elicit (1) a new
// name for the identifier and (2) whether or not occurrences of the name
// should be replaced in comments.  These two inputs are parameters to the
// refactoring.
type Parameter struct {
	// A brief label suitable for display next to an input field (e.g., a
	// text box or check box in a dialog box), e.g., "Name:" or "Replace
	// occurrences"
	Label string
	// A longer (typically one sentence) description of the input
	// requested, suitable for display in a tooltip/hover tip.
	Prompt string
	// The default value for this parameter.  The type of the parameter
	// (string or boolean) can be determined from the type of its default
	// value.
	DefaultValue interface{}
}

// IsBoolean returns true iff this Parameter must be either true or false.
func (p *Parameter) IsBoolean() bool {
	switch p.DefaultValue.(type) {
	case bool:
		return true
	default:
		return false
	}
}

// Description provides information about a refactoring suitable for display in
// a user interface.
type Description struct {
	// A human-readable name for this refactoring, properly capitalized
	// (e.g., "Rename" or "Extract Function") as it would appear in a user
	// interface.  Every refactoring should have a unique name.
	Name string
	// A brief phrase (≤50 characters) describing this refactoring, with
	// the first letter capitalized.  For example:
	//     ----+----1----+----2----+----3----+----4----+----5
	//     Changes the name of an identifier
	Synopsis string
	// A one-line synopsis of this refactoring's arguments (≤50 characters)
	// with angle brackets surrounding argument names (question marks
	// denoting Boolean arguments) and square brackets surrounding optional
	// arguments.  For example:
	//     ----+----1----+----2----+----3----+----4----+----5
	//     <new_name> [<rename_in_comments?>]
	Usage string
	// HTML doumentation for this refactoring, which can be embedded into
	// the User's Guide.
	HTMLDoc string
	// Multifile is set to true only if this refactoring may change files
	// other than the File containing the selection.  For example, renaming
	// an exported identifier may cause references to be updated in several
	// files, so for the Rename refactoring, Multifile=true.  However,
	// extracting a local variable will only change the File containing the
	// selection, so Extract Local Variable has Multifile=false.
	Multifile bool
	// Required inputs for this refactoring (e.g., if a variable is being
	// renamed, a new name for that variable).  See Parameter.
	Params []Parameter
	// Optional inputs following the required inputs.  See Parameter.
	OptionalParams []Parameter
	// False if this refactoring is not intended for production use.
	Hidden bool
}

// A Config provides the initial configuration for a refactoring, including the
// File system and Program on which it will operate, the initial text
// selection, and any refactoring-specific arguments.
//
// At a minimum, the FileSystem, Scope, and Selection arguments must be set.
type Config struct {
	// The File system on which the refactoring will operate.
	FileSystem filesystem.FileSystem
	// A set of initial packages to load.  This extents will be passed as-is
	// to the Config.FromArgs method of go.tools/go/loader.  Typically, the
	// scope will consist of a package name or a File containing the
	// Program entrypoint (main function), which may be different from the
	// File containing the text selection.
	Scope []string
	// The range of text on which to invoke the refactoring.
	Selection text.Selection
	// Refactoring-specific arguments.  To determine what arguments are
	// required for each refactoring, see Refactoring.Description().Params.
	// For example, for the Rename refactoring, you must specify a new name
	// for the entity being renamed.  If the refactoring does not require
	// any arguments, this may be nil.
	Args []interface{}
	// Level of verbosity, i.e., how many types of informational items to
	// include in the log.  If Verbosity ≥ 0, only essential log messages
	// (usually errors and warnings) are included.  If ≥ 1, a list of files
	// modified by the refactoring are added to the log.  If ≥ 2, an
	// exhaustive list of edits made by the refactoring is appended to
	// the log.
	Verbosity int
	// The GOPATH.  If this is set to the empty string, the GOPATH is
	// determined from the environment.
	GoPath string
	// The GOROOT.  If this is set to the empty string, the GOROOT is
	// determined from the environment.
	GoRoot string
}

// The Refactoring interface identifies methods common to all refactorings.
//
// The protocol for invoking a refactoring is:
//
//     1. If necessary, invoke the Description() method to obtain the name of
//        the refactoring and a list of arguments that must be provided to it.
//     2. Create a Config.  Refactorings are typically invoked from a text
//        editor; the Config provides the refactoring with the File that was
//        open in the text editor and the selected region/caret position.
//     3. Invoke Run, which returns a Result.
//     4. If Result.Log is not empty, display the log to the user.
//     5. If Result.Edits is non-nil, the edits may be applied to complete the
//        transformation.
type Refactoring interface {
	Description() *Description
	Run(*Config) *Result
}

type Result struct {
	// A list of informational messages, errors, and warnings to display to
	// the user.  If the Log.ContainsErrors() is true, the Edits may be
	// empty or incomplete, since it may not be possible to perform the
	// refactoring.
	Log *Log
	// Maps filenames to the text edits that should be applied to those
	// files.
	Edits map[string]*text.EditSet
	// DebugOutput is used by the debug refactoring to output additional
	// information (e.g., the description of the AST or control flow graph)
	// that doesn't belong in the log or the output file
	DebugOutput bytes.Buffer
}

const cgoError1 = "could not import C ("
const cgoError2 = "undeclared name: C"

type RefactoringBase struct {
	// The Program to be refactored, including all dependent files
	Program *loader.Program
	// The AST of the File containing the user's selection
	File *ast.File
	// The Filename containing the user's selection
	Filename string
	// The complete contents of the File containing the user's selection
	FileContents []byte
	// The position of the first character of the user's selection
	SelectionStart token.Pos
	// The position immediately following the user's selection
	SelectionEnd token.Pos
	// AST nodes from SelectedNode to the root (from PathEnclosingInterval)
	PathEnclosingSelection []ast.Node
	// Whether the user's selection exactly encloses the SelectedNode (from
	// PathEnclosingInterval)
	SelectionIsExact bool
	// The deepest ast.Node enclosing the user's selection
	SelectedNode ast.Node
	// The package containing the SelectedNode
	SelectedNodePkg *loader.PackageInfo
	// The Result of this refactoring, returned to the client invoking it
	Result
}

// Setup code for a Run method.  Most refactorings should invoke this
// method before performing refactoring-specific work.  This method
// initializes the refactoring, clears the log, and
// configures all of the fields in the RefactoringBase struct.
func (r *RefactoringBase) Init(config *Config, desc *Description) *Result {
	r.Log = NewLog()
	r.Edits = map[string]*text.EditSet{}
	r.DebugOutput.Reset()

	if config.FileSystem == nil {
		r.Log.Error("INTERNAL ERROR: null Config.FileSystem")
		return &r.Result
	}

	if !validateArgs(config, desc, r.Log) {
		return &r.Result
	}

	if config.Scope == nil {
		var msg string
		config.Scope, msg = r.guessScope(config)
		r.Log.Infof(msg)
	} else {
		r.Log.Infof("Scope is %s", strings.Join(config.Scope, " "))
	}

	stdin, _ := filesystem.FakeStdinPath()

	var err error
	mutex := &sync.Mutex{}
	r.Program, err = createLoader(config, func(err error) {
		message := strings.Replace(err.Error(), stdin+":", "<stdin>:", -1)
		// TODO: This is temporary until go/loader handles cgo
		if !strings.Contains(message, cgoError1) &&
			!strings.HasSuffix(message, cgoError2) &&
			len(r.Log.Entries) < maxInitialErrors {
			mutex.Lock()
			if err, ok := err.(types.Error); ok {
				r.Log.Error(err.Msg)
				r.Log.AssociatePos(err.Pos, err.Pos)
			} else {
				r.Log.Error(message)
			}
			mutex.Unlock()
		}
	})

	r.Log.MarkInitial()
	if err != nil {
		r.Log.Error(err)
		return &r.Result
	} else if r.Program == nil {
		r.Log.Error("INTERNAL ERROR: Loader failed")
		return &r.Result
	}

	r.Log.Fset = r.Program.Fset

	r.SelectionStart, r.SelectionEnd, err = config.Selection.Convert(r.Program.Fset)
	if err != nil {
		r.Log.Error(err)
		return &r.Result
	}

	r.SelectedNodePkg, r.PathEnclosingSelection, r.SelectionIsExact =
		r.Program.PathEnclosingInterval(r.SelectionStart, r.SelectionEnd)
	if r.SelectedNodePkg == nil || len(r.PathEnclosingSelection) < 1 {
		r.Log.Errorf("The selected file, %s, was not found in the "+
			"provided scope: %s",
			config.Selection.GetFilename(),
			config.Scope)
		// This can happen on files containing +build
		return &r.Result
	}
	r.SelectedNode = r.PathEnclosingSelection[0]
	r.File = r.PathEnclosingSelection[len(r.PathEnclosingSelection)-1].(*ast.File)

	r.Filename = r.Program.Fset.Position(r.File.Package).Filename

	reader, err := config.FileSystem.OpenFile(r.Filename)
	if err != nil {
		r.Log.Errorf("Unable to open %s", r.Filename)
		return &r.Result
	}
	r.FileContents, err = ioutil.ReadAll(reader)
	if err != nil {
		r.Log.Errorf("Unable to read %s", r.Filename)
		return &r.Result
	}

	/*
		r.Log.Infof("Selection is \"%s\" (offsets %d–%d)",
			r.TextFromPosRange(r.SelectionStart, r.SelectionEnd),
			r.OffsetOfPos(r.SelectionStart),
			r.OffsetOfPos(r.SelectionEnd))
	*/

	r.Edits = map[string]*text.EditSet{
		r.Filename: text.NewEditSet(),
	}

	return &r.Result
}

func createLoader(config *Config, errorHandler func(error)) (*loader.Program, error) {
	buildContext := build.Default
	if os.Getenv("GOPATH") != "" {
		// The test runner may change the GOPATH environment variable
		// since the Program was started, so set it here explicitly
		// (not necessary when run as a CLI tool, but necessary when
		// run from refactoring_test.go)
		buildContext.GOPATH = os.Getenv("GOPATH")
	}
	if config.GoPath != "" {
		buildContext.GOPATH = config.GoPath
	}
	if os.Getenv("GOROOT") != "" {
		// When the Go Doctor Web demo is running on App Engine, the
		// GOROOT environment variable will be set since the default
		// GOROOT is not readable.
		buildContext.GOROOT = os.Getenv("GOROOT")
	}
	if config.GoRoot != "" {
		buildContext.GOROOT = config.GoRoot
	}
	buildContext.ReadDir = config.FileSystem.ReadDir
	buildContext.OpenFile = config.FileSystem.OpenFile

	var lconfig loader.Config
	lconfig.Build = &buildContext
	lconfig.ParserMode = parser.ParseComments | parser.DeclarationErrors
	lconfig.AllowErrors = true
	//lconfig.SourceImports = true
	lconfig.TypeChecker.Error = errorHandler

	rest, err := lconfig.FromArgs(config.Scope, true)
	if len(rest) > 0 {
		errorHandler(fmt.Errorf("Unrecognized argument %s",
			strings.Join(rest, " ")))
	}
	if err != nil {
		errorHandler(err)
	}
	return lconfig.Load()
}

// guessScope makes a reasonable guess at the refactoring scope if the user
// does not provide an explicit scope.  It guesses as follows:
//     1. If Filename is not in $GOPATH/src, Filename is used as the scope.
//     2. If Filename is in $GOPATH/src, a package name is guessed by stripping
//        $GOPATH/src/ from the Filename, and that package is used as the scope.
func (r *RefactoringBase) guessScope(config *Config) ([]string, string) {
	fname := config.Selection.GetFilename()
	fnameScope := []string{fname}

	var fnameMsg string
	if filepath.Base(fname) == filesystem.FakeStdinFilename {
		fnameMsg = "Defaulting to file scope for refactoring (provide an explicit scope to change this)"
	} else {
		fnameMsg = fmt.Sprintf("Defaulting to file scope %s for refactoring (provide an explicit scope to change this)", fname)
	}

	absFilename, err := filepath.Abs(fname)
	if err != nil {
		r.Log.Error(err.Error())
		return fnameScope, fnameMsg
	}

	gopath := config.GoPath
	if gopath == "" {
		gopath = os.Getenv("GOPATH")
	}
	if gopath == "" {
		r.Log.Warn("GOPATH not set")
		return fnameScope, fnameMsg
	}
	gopath, err = filepath.Abs(gopath)
	if err != nil {
		r.Log.Error(err)
		return fnameScope, fnameMsg
	}

	gopathSrc := filepath.Join(gopath, "src")

	relFilename, err := filepath.Rel(gopathSrc, absFilename)
	if err != nil {
		r.Log.Error(err)
		return fnameScope, fnameMsg
	}

	if strings.HasPrefix(relFilename, "..") {
		return fnameScope, fnameMsg
	}

	dir := filepath.Dir(relFilename)
	if dir == "." {
		return fnameScope, fnameMsg
	}

	pkg := filepath.ToSlash(dir)
	return []string{pkg},
		fmt.Sprintf("Defaulting to package scope %s for refactoring (provide an explicit scope to change this)", pkg)
}

// validateArgs determines whether the arguments supplied in the given Config
// match the parameters required by the given Description.  If they mismatch in
// either type or number, a fatal error is logged to the given Log, and the
// function returns false; otherwise, no error is logged, and the function
// returns true.
func validateArgs(config *Config, desc *Description, log *Log) bool {
	minArgsExpected := len(desc.Params)
	maxArgsExpected := len(desc.Params) + len(desc.OptionalParams)
	numArgsSupplied := len(config.Args)
	if numArgsSupplied < minArgsExpected {
		atLeast := ""
		if maxArgsExpected > minArgsExpected {
			atLeast = " at least"
		}
		expectedPlural := "s"
		if minArgsExpected == 1 {
			expectedPlural = ""
		}
		wasWere := "were"
		if numArgsSupplied == 1 {
			wasWere = "was"
		}
		log.Errorf("This refactoring requires%s %d argument%s, "+
			"but %d %s supplied.",
			atLeast,
			minArgsExpected,
			expectedPlural,
			numArgsSupplied,
			wasWere)
		return false
	}
	if numArgsSupplied > maxArgsExpected {
		atMost := ""
		if maxArgsExpected > minArgsExpected {
			atMost = " at most"
		}
		expectedPlural := "s"
		if maxArgsExpected == 1 {
			expectedPlural = ""
		}
		wasWere := "were"
		if numArgsSupplied == 1 {
			wasWere = "was"
		}
		log.Errorf("This refactoring requires%s %d argument%s, "+
			"but %d %s supplied.",
			atMost,
			maxArgsExpected,
			expectedPlural,
			numArgsSupplied,
			wasWere)
		return false
	}

	for i, arg := range config.Args {
		if i < minArgsExpected {
			expected := reflect.TypeOf(desc.Params[i].DefaultValue)
			if reflect.TypeOf(arg) != expected {
				paramName := desc.Params[i].Label
				log.Errorf("%s must be a %s", paramName, expected)
				return false
			}
		} else {
			index := i - minArgsExpected
			expected := reflect.TypeOf(desc.OptionalParams[index].DefaultValue)
			if reflect.TypeOf(arg) != expected {
				paramName := desc.OptionalParams[index].Label
				log.Errorf("%s must be a %s", paramName, expected)
				return false
			}
		}
	}

	return true
}

// lineColToPos converts a line/column position (where the first character in a
// File is at // line 1, column 1) into a token.Pos
func (r *RefactoringBase) lineColToPos(file *ast.File, line int, column int) token.Pos {
	if file == nil {
		panic("file is nil")
	}
	lastLine := -1
	thisColumn := 1
	tfile := r.Program.Fset.File(file.Package)
	for i, size := 0, tfile.Size(); i < size; i++ {
		pos := tfile.Pos(i)
		thisLine := tfile.Line(pos)
		if thisLine != lastLine {
			thisColumn = 1
		} else {
			thisColumn++
		}
		if thisLine == line && thisColumn == column {
			return pos
		}
		lastLine = thisLine
	}
	return file.Pos()
}

func (r *RefactoringBase) FormatFileInEditor() {
	oldFileContents := string(r.FileContents)
	string, err := text.ApplyToString(r.Edits[r.Filename], oldFileContents)
	if err != nil {
		r.Log.Errorf("Transformation produced invalid EditSet: %v",
			err.Error())
		return
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", string, parser.ParseComments)
	if err != nil {
		r.Log.Errorf("Transformation will introduce syntax errors: %v", err)
		r.Log.AssociatePos(r.File.Pos(), r.File.End())
		return
	}

	printConfig := &printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8}
	var b bytes.Buffer
	if err = printConfig.Fprint(&b, fset, file); err != nil {
		r.Log.Error(err)
		return
	}
	newFileContents := b.String()

	editSet := text.Diff(
		strings.SplitAfter(oldFileContents, "\n"),
		strings.SplitAfter(newFileContents, "\n"))
	r.Edits[r.Filename] = editSet
}

// UpdateLog applies the edits in r.Edits and updates existing error messages
// in r.Log to reflect their locations in the resulting Program.  If
// checkForErrors is true, and if the log does not contain any initial errors,
// the resulting Program will be type checked, and any new errors introduced by
// the refactoring will be logged.
func (r *RefactoringBase) UpdateLog(config *Config, checkForErrors bool) {
	if r.Edits == nil || len(r.Edits) == 0 {
		return
	}

	// Avoid loading the refactored Program into a new go/loader if at all
	// possible.  If we won't update the positions of any log entries and
	// won't report any new errors, then we can avoid loading the
	// refactored Program.
	canSkipLoadingProgram := !r.Log.ContainsPositions() && !checkForErrors

	if config.Verbosity == 0 && canSkipLoadingProgram {
		return
	}

	if r.Log.ContainsInitialErrors() {
		checkForErrors = false
	}

	programFiles := map[string]*token.File{}
	r.Program.Fset.Iterate(func(f *token.File) bool {
		programFiles[f.Name()] = f
		return true
	})

	fileCount := len(r.Edits)
	if fileCount >= 2 && config.Verbosity >= 1 {
		fileNum := 1
		for filename, edits := range r.Edits {
			edits.Iterate(func(extent *text.Extent, _ string) bool {
				file := programFiles[filename]
				oldPos := file.Pos(extent.Offset)
				r.Log.Infof("File %d of %d: %s",
					fileNum,
					fileCount,
					filepath.Base(filename))
				r.Log.AssociatePos(oldPos, oldPos)
				fileNum++
				return false
			})
		}
	}

	if config.Verbosity == 1 && canSkipLoadingProgram {
		return
	}

	oldFS := config.FileSystem
	defer func() { config.FileSystem = oldFS }()
	config.FileSystem = filesystem.NewEditedFileSystem(oldFS, r.Edits)

	newLogOldPos := NewLog()
	newLogOldPos.Fset = r.Program.Fset
	newLogNewPos := NewLog()

	stdin, _ := filesystem.FakeStdinPath()

	mutex := &sync.Mutex{}
	errors := 0
	newProg, err := createLoader(config, func(err error) {
		if !checkForErrors {
			return
		}
		message := strings.Replace(err.Error(), stdin+":", "<stdin>:", -1)
		// TODO: This is temporary until go/loader handles cgo
		if !strings.Contains(message, cgoError1) &&
			!strings.HasSuffix(message, cgoError2) &&
			errors < maxInitialErrors {
			mutex.Lock()
			errors++
			msg := fmt.Sprintf("Completing the transformation will introduce the following error: %s", message)
			if err, ok := err.(types.Error); ok {
				newLogOldPos.Error(err.Msg)
				newLogNewPos.Error(err.Msg)
				oldPos := mapPos(err.Fset, err.Pos, r.Edits, programFiles, true)
				newLogOldPos.AssociatePos(oldPos, oldPos)
				newLogNewPos.Fset = err.Fset
				newLogNewPos.AssociatePos(err.Pos, err.Pos)
			} else {
				newLogOldPos.Error(msg)
				newLogNewPos.Error(msg)
			}
			mutex.Unlock()
		}
	})
	if newProg == nil || err != nil {
		r.Log.Append(newLogOldPos.Entries)
		return
	}

	newProgFiles := map[string]*token.File{}
	newProg.Fset.Iterate(func(f *token.File) bool {
		newProgFiles[f.Name()] = f
		return true
	})

	r.Log.Fset = newProg.Fset
	for _, entry := range r.Log.Entries {
		entry.Pos = mapPos(r.Program.Fset, entry.Pos, r.Edits, newProgFiles, false)
	}
	r.Log.Append(newLogNewPos.Entries)

	if config.Verbosity >= 2 {
		for filename, edits := range r.Edits {
			edits.Iterate(func(extent *text.Extent, replace string) bool {
				oldFile := programFiles[filename]
				oldPos := oldFile.Pos(extent.Offset)
				newPos := mapPos(r.Program.Fset, oldPos,
					r.Edits, newProgFiles, false)
				r.Log.Infof(describeEdit(extent, replace))
				r.Log.AssociatePos(newPos, newPos)
				return true
			})
		}
	}
}

// describeEdit returns a human-readable, one-line description of a text edit
func describeEdit(extent *text.Extent, replacement string) string {
	if extent.Length == 0 {
		return fmt.Sprintf("| Insert \"%s\"", shorten(replacement))
	} else if replacement == "" {
		return fmt.Sprintf("| Delete %d byte(s)", extent.Length)
	} else {
		return fmt.Sprintf("| Replace %d byte(s) with \"%s\"",
			extent.Length, shorten(replacement))
	}
}

func shorten(s string) string {
	if len(s) < 23 {
		return s
	}
	return s[:23] + "..."
}

// mapPos takes a Pos in one FileSet and returns the corresponding Pos in
// another FileSet, applying or undoing the given edits (if reverse is false or
// true, respectively) to determine the corresponding offset and comparing
// filenames (as strings) to find the corresponding File.
func mapPos(from *token.FileSet, pos token.Pos, edits map[string]*text.EditSet, toFiles map[string]*token.File, reverse bool) token.Pos {
	if !pos.IsValid() {
		return pos
	}

	filename := from.Position(pos).Filename
	offset := from.Position(pos).Offset
	if es, ok := edits[filename]; ok {
		if reverse {
			offset = es.OldOffset(offset)
		} else {
			offset = es.NewOffset(offset)
		}
	}

	result := token.NoPos
	if file, ok := toFiles[filename]; ok {
		result = file.Pos(offset)
	}
	return result
}

/* -=-=- Utility Methods -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

func InterpretArgs(args []string, r Refactoring) []interface{} {
	params := r.Description().Params
	result := []interface{}{}
	for i, opt := range args {
		if i < len(params) && params[i].IsBoolean() {
			switch opt {
			case "true":
				result = append(result, true)
			case "false":
				result = append(result, false)
			default:
				result = append(result, opt)
			}
		} else {
			result = append(result, opt)
		}
	}
	return result
}

func (r *RefactoringBase) OffsetOfPos(pos token.Pos) int {
	return r.Program.Fset.Position(pos).Offset
}

func (r *RefactoringBase) OffsetLength(node ast.Node) (int, int) {
	offset := r.OffsetOfPos(node.Pos())
	end := r.OffsetOfPos(node.End())
	return offset, end - offset
}

func (r *RefactoringBase) Extent(node ast.Node) *text.Extent {
	offset, length := r.OffsetLength(node)
	return &text.Extent{Offset: offset, Length: length}
}

func (r *RefactoringBase) TextFromPosRange(start, end token.Pos) string {
	fileInEditor := r.Program.Fset.File(r.SelectionStart)
	if r.Program.Fset.File(start) != fileInEditor ||
		r.Program.Fset.File(end) != fileInEditor {
		panic("position must be from file in editor")
	}
	return string(r.FileContents[r.OffsetOfPos(start):r.OffsetOfPos(end)])
}

func (r *RefactoringBase) Text(node ast.Node) string {
	return r.TextFromPosRange(node.Pos(), node.End())
}
