// Copyright 2016-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The cli package provides a command-line interface for the Go Doctor.
package cli

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"strings"

	"github.com/godoctor/godoctor/doc"
	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/engine/protocol"
	"github.com/godoctor/godoctor/filesystem"
	"github.com/godoctor/godoctor/refactoring"
	"github.com/godoctor/godoctor/text"
)

// Usage is the text template used to produce the output of "godoctor -help"
var Usage string

// ensureUsageIsSet sets the Usage template if it has not already been set
// (e.g., in main.go)
func ensureUsageIsSet() {
	if Usage != "" {
		// Usage has already been set by main.go
		return
	}

	numRefactorings := len(engine.AllRefactoringNames())
	if numRefactorings == 1 {
		refac := engine.GetRefactoring(engine.AllRefactoringNames()[0])
		if len(refac.Description().Params) == 0 {
			Usage = `{{.AboutText}}

Usage: {{.CommandName}} [<flag> ...]

Each <flag> must be one of the following:
{{.Flags}}
`
		} else {
			args := refac.Description().Usage
			idx := strings.Index(args, "<")
			if idx < 0 {
				args = ""
			} else {
				args = " " + args[idx:]
			}
			Usage = `{{.AboutText}}

Usage: {{.CommandName}} [<flag> ...]` + args + `

Each <flag> must be one of the following:
{{.Flags}}
`
		}
	} else {
		Usage = `{{.AboutText}} - Go source code refactoring tool.

Usage: {{.CommandName}} [<flag> ...] <refactoring> [<args> ...]

Each <flag> must be one of the following:
{{.Flags}}
The <refactoring> argument determines the refactoring to perform:
{{.Refactorings}}
The <args> following the refactoring name vary depending on the refactoring.
If a refactoring requires arguments but none are supplied, a message will be
displayed with a synopsis of the correct usage.

For complete usage information, see the user manual: http://gorefactor.org/doc.html
`
	}
}

func printHelp(cmdName, aboutText string, flags *flag.FlagSet, stderr io.Writer) {
	var usageFields struct {
		AboutText    string
		CommandName  string
		Flags        string
		Refactorings string
	}

	usageFields.AboutText = aboutText

	usageFields.CommandName = cmdName

	var flagList bytes.Buffer
	flags.VisitAll(func(flag *flag.Flag) {
		fmt.Fprintf(&flagList, "    -%-8s %s\n", flag.Name, flag.Usage)
	})
	usageFields.Flags = flagList.String()

	var refactorings bytes.Buffer
	for _, key := range engine.AllRefactoringNames() {
		r := engine.GetRefactoring(key)
		if !r.Description().Hidden {
			fmt.Fprintf(&refactorings, "    %-15s %s\n",
				key, r.Description().Synopsis)
		}
	}
	usageFields.Refactorings = refactorings.String()

	ensureUsageIsSet()

	t := template.Must(template.New("usage").Parse(Usage))
	err := t.Execute(stderr, usageFields)
	if err != nil {
		fmt.Fprintln(stderr, err)
	}
}

type CLIFlags struct {
	*flag.FlagSet
	fileFlag        *string
	posFlag         *string
	scopeFlag       *string
	completeFlag    *bool
	writeFlag       *bool
	verboseFlag     *bool
	veryVerboseFlag *bool
	listFlag        *bool
	jsonFlag        *bool
	docFlag         *string
}

// Flags returns the flags supported by the godoctor command line tool.
func Flags() *CLIFlags {
	flags := CLIFlags{
		FlagSet: flag.NewFlagSet("godoctor", flag.ContinueOnError)}
	flags.fileFlag = flags.String("file", "",
		"Filename containing an element to refactor (default: stdin)")
	flags.posFlag = flags.String("pos", "1,1:1,1",
		"Position of a syntax element to refactor (default: entire file)")
	flags.scopeFlag = flags.String("scope", "",
		"Package name(s), or source file containing a program entrypoint")
	flags.completeFlag = flags.Bool("complete", false,
		"Output entire modified source files instead of displaying a diff")
	flags.writeFlag = flags.Bool("w", false,
		"Modify source files on disk (write) instead of displaying a diff")
	flags.verboseFlag = flags.Bool("v", false,
		"Verbose: list affected files")
	flags.veryVerboseFlag = flags.Bool("vv", false,
		"Very verbose: list individual edits (implies -v)")
	flags.listFlag = flags.Bool("list", false,
		"List all refactorings and exit")
	flags.jsonFlag = flags.Bool("json", false,
		"Accept commands in OpenRefactory JSON protocol format")
	flags.docFlag = flags.String("doc", "",
		"Output documentation (install, user, man, or vim) and exit")
	return &flags
}

// Run runs the Go Doctor command-line interface.  Typical usage is
//     os.Exit(cli.Run(os.Stdin, os.Stdout, os.Stderr, os.Args))
// All arguments must be non-nil, and args[0] is required.
func Run(aboutText string, stdin io.Reader, stdout io.Writer, stderr io.Writer, args []string) int {
	cmdName := args[0]

	flags := Flags()
	// Don't print full help unless -help was requested.
	// Just gently remind users that it's there.
	flags.Usage = func() {}
	flags.Init(cmdName, flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args[1:]); err != nil {
		// (err has already been printed)
		if err == flag.ErrHelp {
			// Invoked as "godoctor [flags] -help"
			printHelp(cmdName, aboutText, flags.FlagSet, stderr)
			return 2
		}
		fmt.Fprintf(stderr, "Run '%s -help' for more information.\n", cmdName)
		return 1
	}

	args = flags.Args()

	if *flags.docFlag != "" {
		if len(args) > 0 || flags.NFlag() != 1 {
			fmt.Fprintln(stderr, "Error: The -doc flag cannot "+
				"be used with any other flags or arguments")
			return 1
		}
		switch *flags.docFlag {
		case "man":
			doc.PrintManPage(aboutText, flags.FlagSet, stdout)
		case "install":
			doc.PrintInstallGuide(aboutText, flags.FlagSet, stdout)
		case "user":
			doc.PrintUserGuide(aboutText, flags.FlagSet, stdout)
		case "vim":
			doc.PrintVimdoc(aboutText, flags.FlagSet, stdout)
		default:
			fmt.Fprintln(stderr, "Error: The -doc flag must be "+
				"\"man\", \"install\", \"user\", or \"vim\"")
			return 1
		}
		return 0
	}

	if *flags.listFlag {
		if len(args) > 0 {
			fmt.Fprintln(stderr, "Error: The -list flag "+
				"cannot be used with any arguments")
			return 1
		}
		if *flags.verboseFlag || *flags.veryVerboseFlag ||
			*flags.writeFlag || *flags.completeFlag ||
			*flags.jsonFlag {
			fmt.Fprintln(stderr, "Error: The -list flag "+
				"cannot be used with the -v, -vv, -w, "+
				"-complete, or -json flags")
			return 1
		}
		// Invoked: godoctor [-file=""] [-pos=""] [-scope=""] -list
		fmt.Fprintf(stderr, "%-15s\t%-47s\t%s\n",
			"Refactoring", "Description", "     Multifile?")
		fmt.Fprintf(stderr, "--------------------------------------------------------------------------------\n")
		for _, key := range engine.AllRefactoringNames() {
			r := engine.GetRefactoring(key)
			d := r.Description()
			if !r.Description().Hidden {
				fmt.Fprintf(stderr, "%-15s\t%-50s\t%v\n",
					key, d.Synopsis, d.Multifile)
			}
		}
		return 0
	}

	if *flags.jsonFlag {
		if flags.NFlag() != 1 {
			fmt.Fprintln(stderr, "Error: The -json flag "+
				"cannot be used with any other flags")
			return 1
		}
		// Invoked as "godoctor -json [args]
		protocol.Run(os.Stdout, aboutText, args)
		return 0
	}

	if *flags.writeFlag && *flags.completeFlag {
		fmt.Fprintln(stderr, "Error: The -w and -complete flags "+
			"cannot both be present")
		return 1
	}

	if len(args) > 0 && args[0] == "help" {
		// Invoked as "godoctor [flags] help"
		printHelp(cmdName, aboutText, flags.FlagSet, stderr)
		return 2
	}

	var refacName string
	if len(engine.AllRefactoringNames()) == 1 {
		refacName = engine.AllRefactoringNames()[0]
	} else {
		if len(args) == 0 {
			// Invoked as "godoctor [flags]" with no refactoring
			printHelp(cmdName, aboutText, flags.FlagSet, stderr)
			return 2
		}

		refacName = args[0]
		args = args[1:]
	}

	refac := engine.GetRefactoring(refacName)
	if refac == nil {
		fmt.Fprintf(stderr, "There is no refactoring named \"%s\"\n",
			refacName)
		return 1
	}

	if flags.NFlag() == 0 && flags.NArg() == 1 &&
		len(engine.AllRefactoringNames()) != 1 &&
		len(refac.Description().Params) > 0 {
		// Invoked as "godoctor refactoring" but arguments are required
		fmt.Fprintf(stderr, "Usage: %s %s\n",
			refacName, refac.Description().Usage)
		return 2
	}

	stdinPath := ""

	var fileName string
	var fileSystem filesystem.FileSystem
	if *flags.fileFlag != "" && *flags.fileFlag != "-" {
		fileName = *flags.fileFlag
		fileSystem = &filesystem.LocalFileSystem{}
	} else {
		// Filename is - or no filename given; read from standard input
		var err error
		stdinPath, err = filesystem.FakeStdinPath()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fileName = stdinPath
		if *flags.fileFlag == "" {
			fmt.Fprintln(stderr, "Reading Go source code from standard input...")
		}
		bytes, err := ioutil.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fileSystem, err = filesystem.NewSingleEditedFileSystem(
			stdinPath, string(bytes))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	selection, err := text.NewSelection(fileName, *flags.posFlag)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %s.\n", err)
		return 1
	}

	var scope []string
	if *flags.scopeFlag == "" {
		// If no scope provided, let refactoring.go guess the scope
		scope = nil
	} else if *flags.scopeFlag == "-" && stdinPath != "" {
		// Use -scope=- to indicate "stdin file (not package) scope"
		scope = []string{stdinPath}
	} else {
		// Use -scope=a,b,c to specify multiple files/packages
		scope = strings.Split(*flags.scopeFlag, ",")
	}

	verbosity := 0
	if *flags.verboseFlag {
		verbosity = 1
	}
	if *flags.veryVerboseFlag {
		verbosity = 2
	}

	result := refac.Run(&refactoring.Config{
		FileSystem: fileSystem,
		Scope:      scope,
		Selection:  selection,
		Args:       refactoring.InterpretArgs(args, refac),
		Verbosity:  verbosity})

	// Display log in GNU-style 'file:line.col-line.col: message' format
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	result.Log.Write(stderr, cwd)

	// If input was supplied on standard input, ensure that the refactoring
	// makes changes only to that code (and does not affect any other files)
	if stdinPath != "" {
		for f := range result.Edits {
			if f != stdinPath {
				fmt.Fprintf(stderr, "Error: When source code is given on standard input, refactorings are prohibited from changing any other files.  This refactoring would require modifying %s.\n", f)
				return 1
			}
		}
	}

	debugOutput := result.DebugOutput.String()
	if len(debugOutput) > 0 {
		fmt.Fprintln(stdout, debugOutput)
	}

	if *flags.writeFlag {
		err = writeToDisk(result, fileSystem)
	} else if *flags.completeFlag {
		err = writeFileContents(stdout, result.Edits, fileSystem)
	} else {
		err = writeDiff(stdout, result.Edits, fileSystem)
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error: %s.\n", err)
		return 1
	}

	if result.Log.ContainsErrors() {
		return 3
	} else {
		return 0
	}
}

// writeDiff outputs a multi-file unified diff describing this refactoring's
// changes.  It can be applied using GNU patch.
func writeDiff(out io.Writer, edits map[string]*text.EditSet, fs filesystem.FileSystem) error {
	for f, e := range edits {
		p, err := filesystem.CreatePatch(e, fs, f)
		if err != nil {
			return err
		}

		if !p.IsEmpty() {
			inFile := f
			outFile := f
			stdinPath, _ := filesystem.FakeStdinPath()
			if f == stdinPath {
				inFile = os.Stdin.Name()
				outFile = os.Stdout.Name()
			} else {
				rel := relativePath(f)
				inFile = rel
				outFile = rel
			}
			fmt.Fprintf(out, "diff -u %s %s\n", inFile, outFile)
			p.Write(inFile, outFile, time.Time{}, time.Time{}, out)
		}
	}
	return nil
}

// relativePath returns a relative path to fname, or fname if a relative path
// cannot be computed due to an error
func relativePath(fname string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, fname); err == nil {
			return rel
		}
	}
	return fname
}

// writeFileContents outputs the complete contents of each file affected by
// this refactoring.
func writeFileContents(out io.Writer, edits map[string]*text.EditSet, fs filesystem.FileSystem) error {
	for filename, edits := range edits {
		data, err := filesystem.ApplyEdits(edits, fs, filename)
		if err != nil {
			return err
		}

		stdinPath, _ := filesystem.FakeStdinPath()
		if filename == stdinPath {
			filename = os.Stdin.Name()
		}

		if _, err := fmt.Fprintf(out, "@@@@@ %s @@@@@ %d @@@@@\n",
			filename, len(data)); err != nil {
			return err
		}
		n, err := out.Write(data)
		if n < len(data) && err == nil {
			err = io.ErrShortWrite
		}
		if err != nil {
			return err
		}
		if len(data) > 0 && data[len(data)-1] != '\n' {
			fmt.Fprintln(out)
		}
	}
	return nil
}

// writeToDisk overwrites existing files with their refactored versions and
// applies any other changes to the file system that the refactoring requires
// (e.g., renaming directories).
func writeToDisk(result *refactoring.Result, fs filesystem.FileSystem) error {
	for filename, edits := range result.Edits {
		data, err := filesystem.ApplyEdits(edits, fs, filename)
		if err != nil {
			return err
		}

		f, err := fs.OverwriteFile(filename)
		if err != nil {
			return err
		}
		n, err := f.Write(data)
		if err == nil && n < len(data) {
			err = io.ErrShortWrite
		}
		if err1 := f.Close(); err == nil {
			err = err1
		}
		if err != nil {
			return err
		}
	}
	return nil
}
