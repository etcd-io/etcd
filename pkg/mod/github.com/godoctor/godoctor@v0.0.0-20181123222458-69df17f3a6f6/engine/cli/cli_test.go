// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/engine/cli"
	"github.com/godoctor/godoctor/refactoring"
	"github.com/godoctor/godoctor/text"
)

const (
	hello = `package main
import "fmt"
var こんにちはmsg string = "Hello, package"
func main() {
	fmt.Println(こんにちはmsg)
}`
	pos = "-pos=3,5:3,5" // position to rename (msg variable)

	diff = `diff -u /dev/stdin /dev/stdout
--- /dev/stdin
+++ /dev/stdout
@@ -1,6 +1,6 @@
 package main
 import "fmt"
-var こんにちはmsg string = "Hello, package"
+var renamedネーム string = "Hello, package"
 func main() {
-	fmt.Println(こんにちはmsg)
+	fmt.Println(renamedネーム)
 }`

	complete = `@@@@@ /dev/stdin @@@@@ 119 @@@@@
package main
import "fmt"
var renamedネーム string = "Hello, package"
func main() {
	fmt.Println(renamedネーム)
}
`
)

func addRefactoringsAndRunCLI(addRefactorings func(), stdin string, args ...string) (exit int, stdout string, stderr string) {
	args = append(args, "godoctor")
	copy(args[1:], args[0:len(args)-1])
	args[0] = "godoctor"

	var stdoutBuf, stderrBuf bytes.Buffer
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	engine.ClearRefactorings()
	cli.Usage = ""
	addRefactorings()
	exit = cli.Run("Go Doctor TEST", strings.NewReader(stdin),
		&stdoutBuf, &stderrBuf, args)
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	return
}

func runCLI(stdin string, args ...string) (exit int, stdout string, stderr string) {
	return addRefactoringsAndRunCLI(engine.AddDefaultRefactorings, stdin, args...)
}

func TestNoArgsNoInput(t *testing.T) {
	exit, stdout, stderr := runCLI("")
	if exit != 2 || stdout != "" ||
		!strings.Contains(stderr, "Usage: godoctor ") {
		t.Fatal("No args, no input expected usage string with exit 2")
	}
	if !strings.Contains(stderr, "<refactoring>") {
		t.Fatal("Usage message text does not contain <refactoring>")
	}
}

func TestHelp(t *testing.T) {
	for _, helpFlag := range []string{"-help", "--help", "help"} {
		exit, stdout, stderr := runCLI("", helpFlag)
		if exit != 2 || stdout != "" ||
			!strings.Contains(stderr, "Usage: godoctor ") {
			t.Fatalf("%s expected usage string with exit 2", helpFlag)
		}
	}
}

func TestInvalidFlag(t *testing.T) {
	exit, stdout, stderr := runCLI("", "-somethinginvalid")
	if exit != 1 || stdout != "" || stderr == "" {
		t.Fatal("Invalid flag expected exit 1")
	}

	exit, stdout, stderr = runCLI("", "-complete=thisshouldbeaboolean")
	if exit != 1 || stdout != "" || stderr == "" {
		t.Fatal("Invalid flag expected exit 1")
	}
}

func TestDoc(t *testing.T) {
	exit, stdout, stderr := runCLI("", "-doc=man")
	if exit != 0 || stderr != "" || !strings.Contains(stdout, ".TH") {
		t.Fatalf("-doc=man expected man page with exit 0")
	}

	for _, flag := range []string{"-list", "-w", "-complete", "-json"} {
		exit, stdout, stderr = runCLI("", flag, "-doc=man")
		if exit != 1 || stdout != "" || !strings.Contains(stderr,
			"-doc flag cannot be used with") {
			t.Fatalf("-doc should fail and exit 1 if used with %s", flag)
		}
	}
}

func TestList(t *testing.T) {
	exit, stdout, stderr := runCLI("", "-list")
	if exit != 0 || stdout != "" || !strings.Contains(stderr, "rename") {
		t.Fatalf("-list expected refactoring list with exit 0")
	}

	for _, flag := range []string{"-doc=man", "-w", "-complete", "-json"} {
		exit, stdout, stderr = runCLI("", flag, "-list")
		if exit != 1 || stdout != "" || !strings.Contains(stderr,
			"cannot be used with") {
			t.Fatalf("-list should fail and exit 1 if used with %s", flag)
		}
	}
}

func TestInvalidCombos(t *testing.T) {
	invalid := [][]string{
		// complete file json list man pos scope verbose write
		{"-complete", "-json"},
		{"-complete", "-list"},
		{"-complete", "-doc=man"},
		{"-complete", "-w"},
		{"-file=-", "-json"},
		{"-file=-", "-doc=man"},
		{"-json", "-list"},
		{"-json", "-doc=man"},
		{"-json", "-pos=1,1:1,1"},
		{"-json", "-scope=golang.org/x/tools"},
		{"-json", "-v"},
		{"-json", "-w"},
		{"-list", "-doc=man"},
		{"-list", "-v"},
		{"-list", "-w"},
		{"-list", "somearg"},
		{"-doc=man", "-pos=1,1:1,1"},
		{"-doc=man", "-scope=golang.org/x/tools"},
		{"-doc=man", "-v"},
		{"-doc=man", "-w"},
		{"-doc=man", "somearg"},
	}
	for _, flags := range invalid {
		exit, stdout, stderr := runCLI("", flags...)
		if exit != 1 || stdout != "" || !strings.Contains(stderr, "cannot") {
			t.Fatalf("Expected failure and exit 1 if using %s",
				strings.Join(flags, " "))
		}
	}
}

/* FIXME: Enable this after JSON does not fix output to os.Stdout
func TestJSONSmoke(t *testing.T) {
	jsonArg := `[{"command":"list","quality":"in_development"}]`
	exit, stdout, stderr := runCLI("", "-json", jsonArg)
	if exit != 0 || stderr != "" {
		t.Fatalf("-json with argument failed")
	}
	reply := map[string]interface{}{}
	json.Unmarshal([]byte(stdout), &reply)
	if reply["reply"] != "OK" ||
		len(reply["transformations"].([]interface{})) !=
			len(engine.AllRefactorings()) {
		t.Fatalf("JSON expected OK reply, %d refactorings, got %v",
			len(engine.AllRefactorings()), reply["transformations"])
	}
}
*/

func TestInvalidRefactoring(t *testing.T) {
	exit, stdout, stderr := runCLI("", "InvalidRefactoringName")
	if exit != 1 || stdout != "" ||
		!strings.Contains(stderr, "There is no refactoring named") {
		t.Fatal("Invalid refactoring expected exit 1")
	}
}

func TestRefactoringUsage(t *testing.T) {
	exit, stdout, stderr := runCLI("", "rename")
	if exit != 2 || stdout != "" || !strings.Contains(stderr, "Usage:") {
		t.Fatal("\"doctor rename\" expected usage info with exit 2")
	}
}

func TestRenameDiff(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-scope=-", pos, "rename", "renamedネーム")
	if exit != 0 {
		t.Fatalf("Rename expected exit code 0; got %d", exit)
	}
	if stdout != diff {
		t.Fatalf("Output did not match expected diff:\n%s\n%s",
			stdout, stderr)
	}
}

func TestRenameComplete(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-scope=-", pos, "-complete", "rename", "renamedネーム")
	if exit != 0 {
		t.Fatalf("Rename expected exit code 0; got %d", exit)
	}
	if stdout != complete {
		t.Fatalf("Output did not match expected output:\n%s\n%s",
			stdout, stderr)
	}
}

func TestRenameInvalidPos(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-pos=1000,", "rename", "x")
	if exit != 1 || stderr == "" {
		t.Fatal("Rename with invalid position expected error exit 1")
	}
	if stdout != "" {
		t.Fatalf("Rename with invalid -pos should not have output")
	}
}

func TestRenamePosOutOfRange(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-pos=1000,1:1000,1", "rename", "x")
	if exit != 3 || stderr == "" {
		t.Fatalf("Rename position out of range expected exit code 3; got %d", exit)
	}
	if stdout != "" {
		t.Fatalf("Rename position out of range should not have output")
	}
}

func TestRenameInvalidScope(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-scope=invalidScope", "null", "false")
	if exit != 3 || stderr == "" {
		t.Fatalf("Rename with invalid scope should produce exit code 3; got %d", exit)
	}
	if stdout != "" {
		t.Fatalf("Rename with invalid scope should not have output")
	}
}

// Test CLI behavior with a custom set of refactorings (notably, zero or one)

type customNoParams struct{}

func (*customNoParams) Description() *refactoring.Description {
	return &refactoring.Description{
		Name:   "Test",
		Params: nil,
		Hidden: false,
	}
}

func (*customNoParams) Run(config *refactoring.Config) *refactoring.Result {
	return &refactoring.Result{
		Log:   refactoring.NewLog(),
		Edits: map[string]*text.EditSet{},
	}
}

type customOneParam struct {
	refactoring.RefactoringBase
}

func (*customOneParam) Description() *refactoring.Description {
	return &refactoring.Description{
		Name: "Test",
		Params: []refactoring.Parameter{{
			Label:        "Param",
			Prompt:       "Input",
			DefaultValue: "x"}},
		Hidden: false,
	}
}

func (r *customOneParam) Run(config *refactoring.Config) *refactoring.Result {
	return r.Init(config, r.Description())
}

func TestUsageWithNoRefactorings(t *testing.T) {
	exit, stdout, stderr := addRefactoringsAndRunCLI(func() {}, "")
	if exit != 2 || stdout != "" ||
		!strings.Contains(stderr, "Usage: godoctor ") {
		t.Fatal("No args, no input expected usage string with exit 2")
	}
}

func TestUsageWithOneRefactoringNoParams(t *testing.T) {
	exit, stdout, stderr := addRefactoringsAndRunCLI(func() { engine.AddRefactoring("custom", &customNoParams{}) }, "", "-help")
	if exit != 2 || stdout != "" ||
		!strings.Contains(stderr, "Usage: godoctor ") {
		t.Fatal("No args, no input expected usage string with exit 2")
	}
	if strings.Contains(stderr, "<refactoring>") {
		t.Fatal("No args, no input produced usage message for multiple refactorings")
	}
}

func TestUsageWithOneRefactoringOneParam(t *testing.T) {
	exit, stdout, stderr := addRefactoringsAndRunCLI(func() { engine.AddRefactoring("custom", &customOneParam{}) }, "", "-help")
	if exit != 2 || stdout != "" ||
		!strings.Contains(stderr, "Usage: godoctor ") {
		t.Fatal("No args, no input expected usage string with exit 2")
	}
	if strings.Contains(stderr, "<refactoring>") {
		t.Fatal("No args, no input produced usage message for multiple refactorings")
	}
}

func TestOneRefactoringNoParams(t *testing.T) {
	exit, stdout, stderr := addRefactoringsAndRunCLI(func() { engine.AddRefactoring("custom", &customNoParams{}) }, "")
	if exit != 0 || stdout != "" || stderr != "Reading Go source code from standard input...\n" {
		t.Fatal("One refactoring with no args, no input expected exit 0")
	}
}

func TestOneRefactoringNoParamsStdin(t *testing.T) {
	exit, stdout, stderr := addRefactoringsAndRunCLI(func() { engine.AddRefactoring("custom", &customNoParams{}) }, "", "-file=-")
	if exit != 0 || stdout != "" || stderr != "" {
		t.Fatal("One refactoring with no args, no input expected exit 0")
	}
}

func TestOneRefactoringOneParamButNoArgs(t *testing.T) {
	exit, stdout, stderr := addRefactoringsAndRunCLI(func() { engine.AddRefactoring("custom", &customOneParam{}) }, "")
	if exit != 3 || stdout != "" ||
		!strings.Contains(stderr, "Error: This refactoring requires 1 argument, but 0 were supplied.") {
		t.Fatal("One refactoring with one parameter but no args expected error with exit 3")
	}
}

func TestOneRefactoringOneParam(t *testing.T) {
	exit, stdout, _ := addRefactoringsAndRunCLI(func() { engine.AddRefactoring("custom", &customOneParam{}) }, hello, "-scope=-", "x")
	if exit != 0 || stdout != "" {
		t.Fatal("One refactoring with one arg, no input expected exit 0")
	}
}
