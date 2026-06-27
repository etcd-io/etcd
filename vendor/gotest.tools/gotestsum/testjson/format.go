package testjson

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/bitfield/gotestdox"
	"github.com/fatih/color"
)

func debugFormat(out io.Writer) eventFormatterFunc {
	return func(event TestEvent, _ *Execution) error {
		_, err := fmt.Fprintf(out, "%s %s %s (%.3f) [%d] %s\n",
			event.Package,
			event.Test,
			event.Action,
			event.Elapsed,
			event.Time.Unix(),
			event.Output)
		return err
	}
}

// go test -v
func standardVerboseFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	return eventFormatterFunc(func(event TestEvent, _ *Execution) error {
		if event.Action == ActionOutput {
			_, _ = buf.WriteString(event.Output)
			return buf.Flush()
		}
		return nil
	})
}

// go test
func standardQuietFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	return eventFormatterFunc(func(event TestEvent, _ *Execution) error {
		if !event.PackageEvent() {
			return nil
		}
		if event.Output == "PASS\n" {
			return nil
		}

		// Coverage line go1.20+
		if strings.Contains(event.Output, event.Package+"\tcoverage:") {
			return nil
		}
		if isCoverageOutputPreGo119(event.Output) {
			return nil
		}

		if isWarningNoTestsToRunOutput(event.Output) {
			return nil
		}

		_, _ = buf.WriteString(event.Output)
		return buf.Flush()
	})
}

// go test -json
func standardJSONFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	//nolint:errcheck // errors are returned by Flush
	return eventFormatterFunc(func(event TestEvent, _ *Execution) error {
		buf.Write(event.raw)
		buf.WriteRune('\n')
		return buf.Flush()
	})
}

func testNameFormatTestEvent(out io.Writer, event TestEvent) {
	pkgPath := RelativePackagePath(event.Package)

	fmt.Fprintf(out, "%s %s%s (%.2fs)\n",
		colorEvent(event)(strings.ToUpper(string(event.Action))),
		joinPkgToTestName(pkgPath, event.Test),
		formatRunID(event.RunID),
		event.Elapsed)
}

func testDoxFormat(out io.Writer, opts FormatOptions) EventFormatter {
	buf := bufio.NewWriter(out)
	type Result struct {
		Event    TestEvent
		Sentence string
	}

	getIcon := getIconFunc(opts)
	results := map[string][]Result{}
	return eventFormatterFunc(func(event TestEvent, _ *Execution) error {
		switch {
		case event.PackageEvent():
			if !event.Action.IsTerminal() {
				return nil
			}
			if opts.HideEmptyPackages && len(results[event.Package]) == 0 {
				return nil
			}
			fmt.Fprintf(buf, "%s:\n", event.Package)
			tests := results[event.Package]
			sort.Slice(tests, func(i, j int) bool {
				return tests[i].Sentence < tests[j].Sentence
			})
			for _, r := range tests {
				fmt.Fprintf(buf, " %s %s (%.2fs)\n",
					getIcon(r.Event.Action),
					r.Sentence,
					r.Event.Elapsed)
			}
			fmt.Fprintln(buf)
			return buf.Flush()
		case event.Action.IsTerminal():
			// Fuzz test cases tend not to have interesting names,
			// so only report these if they're failures
			if isFuzzCase(event) {
				return nil
			}
			results[event.Package] = append(results[event.Package], Result{
				Event:    event,
				Sentence: gotestdox.Prettify(event.Test),
			})
		}
		return nil
	})
}

func isFuzzCase(event TestEvent) bool {
	return strings.HasPrefix(event.Test, "Fuzz") &&
		event.Action == ActionPass &&
		TestName(event.Test).IsSubTest()
}

func testNameFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)
	//nolint:errcheck
	return eventFormatterFunc(func(event TestEvent, exec *Execution) error {
		formatTest := func() error {
			testNameFormatTestEvent(buf, event)
			return buf.Flush()
		}

		switch {
		case isPkgFailureOutput(event):
			buf.WriteString(event.Output)
			return buf.Flush()

		case event.PackageEvent():
			if !event.Action.IsTerminal() {
				return nil
			}

			result := colorEvent(event)(strings.ToUpper(string(event.Action)))
			pkg := exec.Package(event.Package)
			if event.Action == ActionSkip || (event.Action == ActionPass && pkg.Total == 0) {
				event.Action = ActionSkip // always color these as skip actions
				result = colorEvent(event)("EMPTY")
			}

			event.Elapsed = 0 // hide elapsed for now, for backwards compat
			buf.WriteString(result)
			buf.WriteRune(' ')
			buf.WriteString(packageLine(event, exec.Package(event.Package)))
			return buf.Flush()

		case event.Action == ActionFail:
			pkg := exec.Package(event.Package)
			tc := pkg.LastFailedByName(event.Test)
			pkg.WriteOutputTo(buf, tc.ID)
			return formatTest()

		case event.Action == ActionPass || event.Action == ActionSkip:
			return formatTest()
		}
		return nil
	})
}

// joinPkgToTestName for formatting.
// If the package path isn't the current directory, we add a period to separate
// the test name and the package path. If it is the current directory, we don't
// show it at all. This prevents output like ..MyTest when the test is in the
// current directory.
func joinPkgToTestName(pkg string, test string) string {
	if pkg == "." {
		return test
	}
	return pkg + "." + test
}

// formatRunID returns a formatted string of the runID.
func formatRunID(runID int) string {
	if runID <= 0 {
		return ""
	}
	return fmt.Sprintf(" (re-run %d)", runID)
}

// isPkgFailureOutput returns true if the event is package output, and the output
// doesn't match any of the expected framing messages. Events which match this
// pattern should be package-level failures (ex: exit(1) or panic in an init() or
// TestMain).
func isPkgFailureOutput(event TestEvent) bool {
	out := event.Output
	return all(
		event.PackageEvent(),
		event.Action == ActionOutput,
		out != "PASS\n",
		out != "FAIL\n",
		!isWarningNoTestsToRunOutput(out),
		!strings.HasPrefix(out, "FAIL\t"+event.Package),
		!strings.HasPrefix(out, "ok  \t"+event.Package),
		!strings.HasPrefix(out, "?   \t"+event.Package),
		!isShuffleSeedOutput(out),
	)
}

func all(cond ...bool) bool {
	for _, c := range cond {
		if !c {
			return false
		}
	}
	return true
}

func pkgNameFormat(out io.Writer, opts FormatOptions) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			return nil
		}
		_, _ = buf.WriteString(shortFormatPackageEvent(opts, event, exec))
		return buf.Flush()
	}
}

type icons struct {
	pass  string
	fail  string
	skip  string
	color bool
}

func (i icons) forAction(action Action) string {
	if i.color {
		switch action {
		case ActionPass:
			return color.GreenString(i.pass)
		case ActionSkip:
			return color.YellowString(i.skip)
		case ActionFail:
			return color.RedString(i.fail)
		default:
			return " "
		}
	} else {
		switch action {
		case ActionPass:
			return i.pass
		case ActionSkip:
			return i.skip
		case ActionFail:
			return i.fail
		default:
			return " "
		}
	}
}

func getIconFunc(opts FormatOptions) func(Action) string {
	switch {
	case opts.UseHiVisibilityIcons || opts.Icons == "hivis":
		return icons{
			pass:  "✅", // WHITE HEAVY CHECK MARK
			skip:  "➖", // HEAVY MINUS SIGN
			fail:  "❌", // CROSS MARK
			color: false,
		}.forAction
	case opts.Icons == "text":
		return icons{
			pass:  "PASS",
			skip:  "SKIP",
			fail:  "FAIL",
			color: true,
		}.forAction
	case opts.Icons == "codicons":
		return icons{
			pass:  "\ueba4", // cod-pass
			skip:  "\ueabd", // cod-circle_slash
			fail:  "\uea87", // cod-error
			color: true,
		}.forAction
	case opts.Icons == "octicons":
		return icons{
			pass:  "\uf49e", // oct-check_circle
			skip:  "\uf517", // oct-skip
			fail:  "\uf52f", // oct-x_circle
			color: true,
		}.forAction
	case opts.Icons == "emoticons":
		return icons{
			pass:  "\U000f01f5", // md-emoticon_happy_outline
			skip:  "\U000f01f6", // md-emoticon_neutral_outline
			fail:  "\U000f01f8", // md-emoticon_sad_outline
			color: true,
		}.forAction
	default:
		return icons{
			pass:  "✓", // CHECK MARK
			skip:  "∅", // EMPTY SET
			fail:  "✖", // HEAVY MULTIPLICATION X
			color: true,
		}.forAction
	}
}

func shortFormatPackageEvent(opts FormatOptions, event TestEvent, exec *Execution) string {
	pkg := exec.Package(event.Package)

	getIcon := getIconFunc(opts)
	fmtEvent := func(action string) string {
		return action + "  " + packageLine(event, exec.Package(event.Package))
	}
	switch event.Action {
	case ActionSkip:
		if opts.HideEmptyPackages {
			return ""
		}
		return fmtEvent(getIcon(event.Action))
	case ActionPass:
		if pkg.Total == 0 {
			if opts.HideEmptyPackages {
				return ""
			}
			return fmtEvent(getIcon(ActionSkip))
		}
		return fmtEvent(getIcon(event.Action))
	case ActionFail:
		return fmtEvent(getIcon(event.Action))
	}
	return ""
}

func packageLine(event TestEvent, pkg *Package) string {
	var buf strings.Builder
	buf.WriteString(RelativePackagePath(event.Package))

	switch {
	case pkg.cached:
		buf.WriteString(" (cached)")
	case event.Elapsed != 0:
		d := elapsedDuration(event.Elapsed)
		buf.WriteString(fmt.Sprintf(" (%s)", d))
	}

	if pkg.coverage != "" {
		buf.WriteString(" (" + pkg.coverage + ")")
	}

	if event.Action == ActionFail && pkg.shuffleSeed != "" {
		buf.WriteString(" (" + pkg.shuffleSeed + ")")
	}
	buf.WriteString("\n")
	return buf.String()
}

func pkgNameWithFailuresFormat(out io.Writer, opts FormatOptions) eventFormatterFunc {
	buf := bufio.NewWriter(out)
	return func(event TestEvent, exec *Execution) error {
		if !event.PackageEvent() {
			if event.Action == ActionFail {
				pkg := exec.Package(event.Package)
				tc := pkg.LastFailedByName(event.Test)
				pkg.WriteOutputTo(buf, tc.ID) //nolint:errcheck
				return buf.Flush()
			}
			return nil
		}
		buf.WriteString(shortFormatPackageEvent(opts, event, exec))
		return buf.Flush()
	}
}

func colorEvent(event TestEvent) func(format string, a ...interface{}) string {
	switch event.Action {
	case ActionPass:
		return color.GreenString
	case ActionFail:
		return color.RedString
	case ActionSkip:
		return color.YellowString
	}
	return color.WhiteString
}

// EventFormatter is a function which handles an event and returns a string to
// output for the event.
type EventFormatter interface {
	Format(event TestEvent, output *Execution) error
}

type eventFormatterFunc func(event TestEvent, output *Execution) error

func (e eventFormatterFunc) Format(event TestEvent, output *Execution) error {
	return e(event, output)
}

type FormatOptions struct {
	HideEmptyPackages    bool
	UseHiVisibilityIcons bool // Deprecated
	Icons                string
}

// NewEventFormatter returns a formatter for printing events.
func NewEventFormatter(out io.Writer, format string, formatOpts FormatOptions) EventFormatter {
	switch format {
	case "none":
		return eventFormatterFunc(func(TestEvent, *Execution) error { return nil })
	case "debug":
		return debugFormat(out)
	case "standard-json":
		return standardJSONFormat(out)
	case "standard-verbose":
		return standardVerboseFormat(out)
	case "standard-quiet":
		return standardQuietFormat(out)
	case "dots", "dots-v1":
		return dotsFormatV1(out)
	case "dots-v2":
		return newDotFormatter(out, formatOpts)
	case "gotestdox", "testdox":
		return testDoxFormat(out, formatOpts)
	case "testname", "short-verbose":
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			return githubActionsFormat(out)
		}
		return testNameFormat(out)
	case "pkgname", "short":
		return pkgNameFormat(out, formatOpts)
	case "pkgname-and-test-fails", "short-with-failures":
		return pkgNameWithFailuresFormat(out, formatOpts)
	case "github-actions", "github-action":
		return githubActionsFormat(out)
	default:
		return nil
	}
}

func githubActionsFormat(out io.Writer) EventFormatter {
	buf := bufio.NewWriter(out)

	type name struct {
		Package string
		Test    string
	}
	output := map[name][]string{}

	return eventFormatterFunc(func(event TestEvent, exec *Execution) error {
		key := name{Package: event.Package, Test: event.Test}

		// test case output
		if event.Test != "" && event.Action == ActionOutput {
			if !isFramingLine(event.Output, event.Test) {
				output[key] = append(output[key], event.Output)
			}
			return nil
		}

		// test case end event
		if event.Test != "" && event.Action.IsTerminal() {
			if len(output[key]) > 0 {
				buf.WriteString("::group::")
			} else {
				buf.WriteString("  ")
			}
			testNameFormatTestEvent(buf, event)

			for _, item := range output[key] {
				buf.WriteString(item)
			}
			if len(output[key]) > 0 {
				buf.WriteString("\n::endgroup::\n")
			}
			delete(output, key)
			return buf.Flush()
		}

		// package event
		if !event.Action.IsTerminal() {
			return nil
		}

		result := colorEvent(event)(strings.ToUpper(string(event.Action)))
		pkg := exec.Package(event.Package)
		if event.Action == ActionSkip || (event.Action == ActionPass && pkg.Total == 0) {
			event.Action = ActionSkip // always color these as skip actions
			result = colorEvent(event)("EMPTY")
		}

		buf.WriteString("  ")
		buf.WriteString(result)
		buf.WriteString(" Package ")
		buf.WriteString(packageLine(event, exec.Package(event.Package)))
		buf.WriteString("\n")
		return buf.Flush()
	})
}
