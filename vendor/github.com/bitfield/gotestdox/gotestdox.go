package gotestdox

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

const Usage = `gotestdox is a command-line tool for turning Go test names into readable sentences.

Usage:

	gotestdox [ARGS]

This will run 'go test -json [ARGS]' in the current directory and format the results in a readable
way. You can use any arguments that 'go test -json' accepts, including a list of packages, for
example.

If the standard input is not an interactive terminal, gotestdox will assume you want to pipe JSON
data into it. For example:

	go test -json |gotestdox

See https://github.com/bitfield/gotestdox for more information.`

// Main runs the command-line interface for gotestdox. The exit status for the
// binary is 0 if the tests passed, or 1 if the tests failed, or there was some
// error.
func Main() int {
	if len(os.Args) > 1 && os.Args[1] == "-h" {
		fmt.Println(Usage)
		return 0
	}
	td := NewTestDoxer()
	if isatty.IsTerminal(os.Stdin.Fd()) {
		td.ExecGoTest(os.Args[1:])
	} else {
		td.Filter()
	}
	if !td.OK {
		return 1
	}
	return 0
}

// TestDoxer holds the state and config associated with a particular invocation
// of 'go test'.
type TestDoxer struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	OK             bool
}

// NewTestDoxer returns a [*TestDoxer] configured with the default I/O streams:
// [os.Stdin], [os.Stdout], and [os.Stderr].
func NewTestDoxer() *TestDoxer {
	return &TestDoxer{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// ExecGoTest runs the 'go test -json' command, with any extra args supplied by
// the user, and consumes its output. Any errors are reported to td's Stderr
// stream, including the full command line that was run. If all tests passed,
// td.OK will be true. If there was a test failure, or 'go test' returned some
// error, then td.OK will be false.
func (td *TestDoxer) ExecGoTest(userArgs []string) {
	args := []string{"test", "-json"}
	args = append(args, userArgs...)
	cmd := exec.Command("go", args...)
	goTestOutput, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(td.Stderr, cmd.Args, err)
		return
	}
	cmd.Stderr = td.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(td.Stderr, cmd.Args, err)
		return
	}
	td.Stdin = goTestOutput
	td.Filter()
	if err := cmd.Wait(); err != nil {
		td.OK = false
		fmt.Fprintln(td.Stderr, cmd.Args, err)
		return
	}
}

// Filter reads from td's Stdin stream, line by line, processing JSON records
// emitted by 'go test -json'.
//
// For each Go package it sees records about, it will print the full name of
// the package to td.Stdout, followed by a line giving the pass/fail status and
// the prettified name of each test, sorted alphabetically.
//
// If all tests passed, td.OK will be true at the end. If not, or if there was
// a parsing error, it will be false. Errors will be reported to td.Stderr.
func (td *TestDoxer) Filter() {
	td.OK = true
	results := map[string][]Event{}
	outputs := map[string][]string{}
	scanner := bufio.NewScanner(td.Stdin)
	for scanner.Scan() {
		event, err := ParseJSON(scanner.Text())
		if err != nil {
			td.OK = false
			fmt.Fprintln(td.Stderr, err)
			return
		}
		switch {
		case event.IsPackageResult():
			fmt.Fprintf(td.Stdout, "%s:\n", event.Package)
			tests := results[event.Package]
			sort.Slice(tests, func(i, j int) bool {
				return tests[i].Sentence < tests[j].Sentence
			})
			for _, r := range tests {
				fmt.Fprintln(td.Stdout, r.String())
				if r.Action == ActionFail {
					for _, line := range outputs[r.Test] {
						fmt.Fprint(td.Stdout, line)
					}
				}
			}
			fmt.Fprintln(td.Stdout)
		case event.IsOutput():
			outputs[event.Test] = append(outputs[event.Test], event.Output)
		case event.IsTestResult(), event.IsFuzzFail():
			event.Sentence = Prettify(event.Test)
			results[event.Package] = append(results[event.Package], event)
			if event.Action == ActionFail {
				td.OK = false
			}
		}
	}
}

// ParseJSON takes a string representing a single JSON test record as emitted
// by 'go test -json', and attempts to parse it into an [Event], returning any
// parsing error encountered.
func ParseJSON(line string) (Event, error) {
	event := Event{}
	err := json.Unmarshal([]byte(line), &event)
	if err != nil {
		return Event{}, fmt.Errorf("parsing JSON: %w\ninput: %s", err, line)
	}
	return event, nil
}

const (
	ActionPass = "pass"
	ActionFail = "fail"
)

// Event represents a Go test event as recorded by the 'go test -json' command.
// It does not attempt to unmarshal all the data, only those fields it needs to
// know about. It is based on the (unexported) 'event' struct used by Go's
// [cmd/internal/test2json] package.
type Event struct {
	Action   string
	Package  string
	Test     string
	Sentence string
	Output   string
	Elapsed  float64
}

// String formats a test Event for display. The prettified test name will be
// prefixed by a ✔ if the test passed, or an x if it failed.
//
// The sentence generated by [Prettify] from the name of the test will be
// shown, followed by the elapsed time in parentheses, to 2 decimal places.
//
// # Colour
//
// If the program is attached to an interactive terminal, as determined by
// [github.com/mattn/go-isatty], and the NO_COLOR environment variable is not
// set, check marks will be shown in green and x's in red.
func (e Event) String() string {
	status := color.RedString("x")
	if e.Action == ActionPass {
		status = color.GreenString("✔")
	}
	return fmt.Sprintf(" %s %s (%.2fs)", status, e.Sentence, e.Elapsed)
}

// IsTestResult determines whether or not the test event is one that we are
// interested in (namely, a pass or fail event on a test). Events on non-tests
// (for example, examples) are ignored, and all events on tests other than pass
// or fail events (for example, run or pause events) are also ignored.
func (e Event) IsTestResult() bool {
	// Skip events on benchmarks, examples, and fuzz tests
	if strings.HasPrefix(e.Test, "Benchmark") {
		return false
	}
	if strings.HasPrefix(e.Test, "Example") {
		return false
	}
	if strings.HasPrefix(e.Test, "Fuzz") {
		return false
	}
	if e.Test == "" {
		return false
	}
	if e.Action == ActionPass || e.Action == ActionFail {
		return true
	}
	return false
}

func (e Event) IsFuzzFail() bool {
	if !strings.HasPrefix(e.Test, "Fuzz") {
		return false
	}
	if e.Action != ActionFail {
		return false
	}
	return true
}

// IsPackageResult determines whether or not the test event is a package pass
// or fail event. That is, whether it indicates the passing or failing of a
// package as a whole, rather than some individual test within the package.
func (e Event) IsPackageResult() bool {
	if e.Test != "" {
		return false
	}
	if e.Action == ActionPass || e.Action == ActionFail {
		return true
	}
	return false
}

// IsOutput determines whether or not the event is a test output (for example
// from [testing.T.Error]), excluding status messages automatically generated
// by 'go test' such as "--- FAIL: ..." or "=== RUN / PAUSE / CONT".
func (e Event) IsOutput() bool {
	if e.Action != "output" {
		return false
	}
	if strings.HasPrefix(e.Output, "---") {
		return false
	}
	if strings.HasPrefix(e.Output, "===") {
		return false
	}
	return true
}
