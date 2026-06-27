package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/dnephin/pflag"
	"github.com/fatih/color"
	"gotest.tools/gotestsum/internal/log"
	"gotest.tools/gotestsum/testjson"
)

func Run(name string, args []string) error {
	flags, opts := setupFlags(name)
	switch err := flags.Parse(args); {
	case err == pflag.ErrHelp:
		return nil
	case err != nil:
		usage(os.Stderr, name, flags)
		return err
	}
	opts.args = flags.Args()
	setupLogging(opts)

	switch {
	case opts.version:
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return fmt.Errorf("failed to read version info")
		}
		fmt.Fprintf(os.Stdout, "gotestsum version %s\n", info.Main.Version)
		return nil
	case opts.watch:
		return runWatcher(opts)
	}
	return run(opts)
}

func setupFlags(name string) (*pflag.FlagSet, *options) {
	opts := &options{
		hideSummary:                  newHideSummaryValue(),
		junitTestCaseClassnameFormat: &junitFieldFormatValue{},
		junitTestSuiteNameFormat:     &junitFieldFormatValue{},
		postRunHookCmd:               &commandValue{},
		stdout:                       color.Output,
		stderr:                       color.Error,
	}
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetInterspersed(false)
	flags.Usage = func() {
		usage(os.Stdout, name, flags)
	}

	flags.StringVarP(&opts.format, "format", "f",
		lookEnvWithDefault("GOTESTSUM_FORMAT", "pkgname"),
		"print format of test input")
	flags.BoolVar(&opts.formatOptions.HideEmptyPackages, "format-hide-empty-pkg",
		false, "do not print empty packages in compact formats")
	flags.BoolVar(&opts.formatOptions.UseHiVisibilityIcons, "format-hivis",
		false, "use high visibility characters in some formats")
	_ = flags.MarkHidden("format-hivis")
	flags.StringVar(&opts.formatOptions.Icons, "format-icons",
		lookEnvWithDefault("GOTESTSUM_FORMAT_ICONS", ""),
		"use different icons, see help for options")
	flags.BoolVar(&opts.rawCommand, "raw-command", false,
		"don't prepend 'go test -json' to the 'go test' command")
	flags.BoolVar(&opts.ignoreNonJSONOutputLines, "ignore-non-json-output-lines", false,
		"write non-JSON 'go test' output lines to stderr instead of failing")
	flags.Lookup("ignore-non-json-output-lines").Hidden = true
	flags.StringVar(&opts.jsonFile, "jsonfile",
		lookEnvWithDefault("GOTESTSUM_JSONFILE", ""),
		"write all TestEvents to file")
	flags.StringVar(&opts.jsonFileTimingEvents, "jsonfile-timing-events",
		lookEnvWithDefault("GOTESTSUM_JSONFILE_TIMING_EVENTS", ""),
		"write only the pass, skip, and fail TestEvents to the file")
	flags.BoolVar(&opts.noColor, "no-color", defaultNoColor(), "disable color output")

	flags.Var(opts.hideSummary, "no-summary",
		"do not print summary of: "+testjson.SummarizeAll.String())
	flags.Lookup("no-summary").Hidden = true
	flags.Var(opts.hideSummary, "hide-summary",
		"hide sections of the summary: "+testjson.SummarizeAll.String())
	flags.Var(opts.postRunHookCmd, "post-run-command",
		"command to run after the tests have completed")
	flags.BoolVar(&opts.watch, "watch", false,
		"watch go files, and run tests when a file is modified")
	flags.BoolVar(&opts.watchClear, "watch-clear", false,
		"in watch mode clear screen when rerun tests")
	flags.BoolVar(&opts.watchChdir, "watch-chdir", false,
		"in watch mode change the working directory to the directory with the modified file before running tests")
	flags.IntVar(&opts.maxFails, "max-fails", 0,
		"end the test run after this number of failures")

	flags.StringVar(&opts.junitFile, "junitfile",
		lookEnvWithDefault("GOTESTSUM_JUNITFILE", ""),
		"write a JUnit XML file")
	flags.Var(opts.junitTestSuiteNameFormat, "junitfile-testsuite-name",
		"format the testsuite name field as: "+junitFieldFormatValues)
	flags.Var(opts.junitTestCaseClassnameFormat, "junitfile-testcase-classname",
		"format the testcase classname field as: "+junitFieldFormatValues)
	flags.StringVar(&opts.junitProjectName, "junitfile-project-name",
		lookEnvWithDefault("GOTESTSUM_JUNITFILE_PROJECT_NAME", ""),
		"name of the project used in the junit.xml file")
	flags.BoolVar(&opts.junitHideEmptyPackages, "junitfile-hide-empty-pkg",
		truthyFlag(lookEnvWithDefault("GOTESTSUM_JUNIT_HIDE_EMPTY_PKG", "")),
		"omit packages with no tests from the junit.xml file")
	flags.BoolVar(&opts.junitHideSkippedTests, "junitfile-hide-skipped-tests",
		truthyFlag(lookEnvWithDefault("GOTESTSUM_JUNIT_HIDE_SKIPPED_TESTS", "")),
		"omit skipped tests from the junit.xml file")

	flags.IntVar(&opts.rerunFailsMaxAttempts, "rerun-fails", 0,
		"rerun failed tests until they all pass, or attempts exceeds maximum. Defaults to max 2 reruns when enabled")
	flags.BoolVar(&opts.rerunFailsAbortOnDataRace, "rerun-fails-abort-on-data-race", false,
		"do not rerun tests if a data race is detected")
	flags.Lookup("rerun-fails").NoOptDefVal = "2"
	flags.IntVar(&opts.rerunFailsMaxInitialFailures, "rerun-fails-max-failures", 10,
		"do not rerun any tests if the initial run has more than this number of failures")
	flags.Var((*stringSlice)(&opts.packages), "packages",
		"space separated list of package to test")
	flags.StringVar(&opts.rerunFailsReportFile, "rerun-fails-report", "",
		"write a report to the file, of the tests that were rerun")
	flags.BoolVar(&opts.rerunFailsRunRootCases, "rerun-fails-run-root-test", false,
		"rerun the entire root testcase when any of its subtests fail, instead of only the failed subtest")

	flags.BoolVar(&opts.debug, "debug", false, "enabled debug logging")
	flags.BoolVar(&opts.version, "version", false, "show version and exit")
	return flags, opts
}

func usage(out io.Writer, name string, flags *pflag.FlagSet) {
	fmt.Fprintf(out, `Usage:
    %[1]s [flags] [--] [go test flags]
    %[1]s [command]

See https://pkg.go.dev/gotest.tools/gotestsum#section-readme for detailed documentation.

Flags:
`, name)
	flags.SetOutput(out)
	flags.PrintDefaults()
	fmt.Fprintf(out, `
Formats:
    dots                     print a character for each test
    dots-v2                  experimental dots format, one package per line
    pkgname                  print a line for each package
    pkgname-and-test-fails   print a line for each package and failed test output
    testname                 print a line for each test and package
    testdox                  print a sentence for each test using gotestdox
    github-actions           testname format with github actions log grouping
    standard-quiet           standard go test format
    standard-verbose         standard go test -v format

Format icons:
    default                  the original unicode (✓, ∅, ✖)
    hivis                    higher visibility unicode (✅, ➖, ❌)
    text                     simple text characters (PASS, SKIP, FAIL)
    codicons                 requires a font from https://www.nerdfonts.com/ (  )
    octicons                 requires a font from https://www.nerdfonts.com/ (  )
    emoticons                requires a font from https://www.nerdfonts.com/ (󰇵 󰇶 󰇸)

Commands:
    %[1]s tool slowest   find or skip the slowest tests
    %[1]s help           print this help text
`, name)
}

func lookEnvWithDefault(key, defValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defValue
}

type options struct {
	args                         []string
	format                       string
	formatOptions                testjson.FormatOptions
	debug                        bool
	rawCommand                   bool
	ignoreNonJSONOutputLines     bool
	jsonFile                     string
	jsonFileTimingEvents         string
	junitFile                    string
	postRunHookCmd               *commandValue
	noColor                      bool
	hideSummary                  *hideSummaryValue
	junitTestSuiteNameFormat     *junitFieldFormatValue
	junitTestCaseClassnameFormat *junitFieldFormatValue
	junitProjectName             string
	junitHideEmptyPackages       bool
	junitHideSkippedTests        bool
	rerunFailsMaxAttempts        int
	rerunFailsMaxInitialFailures int
	rerunFailsReportFile         string
	rerunFailsRunRootCases       bool
	rerunFailsAbortOnDataRace    bool
	packages                     []string
	watch                        bool
	watchClear                   bool
	watchChdir                   bool
	maxFails                     int
	version                      bool

	// shims for testing
	stdout io.Writer
	stderr io.Writer
}

func (o options) Validate() error {
	if o.rerunFailsMaxAttempts > 0 && len(o.args) > 0 && !o.rawCommand && len(o.packages) == 0 {
		return fmt.Errorf(
			"when go test args are used with --rerun-fails " +
				"the list of packages to test must be specified by the --packages flag")
	}
	if o.rerunFailsMaxAttempts > 0 &&
		(boolArgIndex("failfast", o.args) > -1 ||
			boolArgIndex("test.failfast", o.args) > -1) {
		return fmt.Errorf("-(test.)failfast can not be used with --rerun-fails " +
			"because not all test cases will run")
	}
	return nil
}

func defaultNoColor() bool {
	// fatih/color will only output color when stdout is a terminal which is not
	// true for many CI environments which support color output. So instead, we
	// try to detect these CI environments via their environment variables.
	// This code is based on https://github.com/jwalton/go-supportscolor
	if value, exists := os.LookupEnv("CI"); exists {
		var ciEnvNames = []string{
			"APPVEYOR",
			"BUILDKITE",
			"CIRCLECI",
			"DRONE",
			"GITEA_ACTIONS",
			"GITHUB_ACTIONS",
			"GITLAB_CI",
			"TRAVIS",
		}
		for _, ciEnvName := range ciEnvNames {
			if _, exists := os.LookupEnv(ciEnvName); exists {
				return false
			}
		}
		if os.Getenv("CI_NAME") == "codeship" {
			return false
		}
		if value == "woodpecker" {
			return false
		}
	}
	if _, exists := os.LookupEnv("TEAMCITY_VERSION"); exists {
		return false
	}
	return color.NoColor
}

func setupLogging(opts *options) {
	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}
	color.NoColor = opts.noColor
}

func run(opts *options) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := opts.Validate(); err != nil {
		return err
	}

	goTestProc, err := startGoTestFn(ctx, "", goTestCmdArgs(opts, rerunOpts{}))
	if err != nil {
		return err
	}

	handler, err := newEventHandler(opts)
	if err != nil {
		return err
	}
	defer handler.Close() //nolint:errcheck
	cfg := testjson.ScanConfig{
		Stdout:                   goTestProc.stdout,
		Stderr:                   goTestProc.stderr,
		Handler:                  handler,
		Stop:                     cancel,
		IgnoreNonJSONOutputLines: opts.ignoreNonJSONOutputLines,
	}
	exec, err := testjson.ScanTestOutput(cfg)
	handler.Flush()
	if err != nil {
		return finishRun(opts, exec, err)
	}

	exitErr := goTestProc.cmd.Wait()
	if signum := atomic.LoadInt32(&goTestProc.signal); signum != 0 {
		return finishRun(opts, exec, exitError{num: signalExitCode + int(signum)})
	}
	if exitErr == nil || opts.rerunFailsMaxAttempts == 0 {
		return finishRun(opts, exec, exitErr)
	}
	if err := hasErrors(exitErr, exec, opts); err != nil {
		return finishRun(opts, exec, err)
	}

	failed := len(rerunFailsFilter(opts)(exec.Failed()))
	if failed > opts.rerunFailsMaxInitialFailures {
		err := fmt.Errorf(
			"number of test failures (%d) exceeds maximum (%d) set by --rerun-fails-max-failures",
			failed, opts.rerunFailsMaxInitialFailures)
		return finishRun(opts, exec, err)
	}

	cfg = testjson.ScanConfig{Execution: exec, Handler: handler}
	exitErr = rerunFailed(ctx, opts, cfg)
	handler.Flush()
	if err := writeRerunFailsReport(opts, exec); err != nil {
		return err
	}
	return finishRun(opts, exec, exitErr)
}

func finishRun(opts *options, exec *testjson.Execution, exitErr error) error {
	testjson.PrintSummary(opts.stdout, exec, opts.hideSummary.value)

	if err := writeJUnitFile(opts, exec); err != nil {
		return fmt.Errorf("failed to write junit file: %w", err)
	}
	if err := postRunHook(opts, exec); err != nil {
		return fmt.Errorf("post run command failed: %w", err)
	}
	return exitErr
}

func goTestCmdArgs(opts *options, rerunOpts rerunOpts) []string {
	if opts.rawCommand {
		var result []string
		result = append(result, opts.args...)
		result = append(result, rerunOpts.Args()...)
		return result
	}

	args := append([]string{}, opts.args...)
	result := []string{"go", "test"}

	if len(args) == 0 {
		result = append(result, "-json")
		if rerunOpts.runFlag != "" {
			result = append(result, rerunOpts.runFlag)
		}
		return append(result, cmdArgPackageList(opts, rerunOpts, "./...")...)
	}

	if boolArgIndex("json", args) < 0 {
		result = append(result, "-json")
	}

	if rerunOpts.runFlag != "" {
		// Remove any existing run arg, it needs to be replaced with our new one
		// and duplicate args are not allowed by 'go test'.
		runIndex, runIndexEnd := argIndex("run", args)
		if runIndex >= 0 && runIndexEnd < len(args) {
			args = append(args[:runIndex], args[runIndexEnd+1:]...)
		}
		result = append(result, rerunOpts.runFlag)
	}

	pkgArgIndex := findPkgArgPosition(args)
	result = append(result, args[:pkgArgIndex]...)
	result = append(result, cmdArgPackageList(opts, rerunOpts)...)
	result = append(result, args[pkgArgIndex:]...)
	return result
}

func cmdArgPackageList(opts *options, rerunOpts rerunOpts, defPkgList ...string) []string {
	switch {
	case rerunOpts.pkg != "":
		return []string{rerunOpts.pkg}
	case len(opts.packages) > 0:
		return opts.packages
	case os.Getenv("TEST_DIRECTORY") != "":
		return []string{os.Getenv("TEST_DIRECTORY")}
	default:
		return defPkgList
	}
}

func boolArgIndex(flag string, args []string) int {
	for i, arg := range args {
		if arg == "-"+flag || arg == "--"+flag {
			return i
		}
	}
	return -1
}

func argIndex(flag string, args []string) (start, end int) {
	for i, arg := range args {
		if arg == "-"+flag || arg == "--"+flag {
			return i, i + 1
		}
		if strings.HasPrefix(arg, "-"+flag+"=") || strings.HasPrefix(arg, "--"+flag+"=") {
			return i, i
		}
	}
	return -1, -1
}

// The package list is before the -args flag, or at the end of the args list
// if the -args flag is not in args.
// The -args flag is a 'go test' flag that indicates that all subsequent
// args should be passed to the test binary. It requires that the list of
// packages comes before -args, so we re-use it as a placeholder in the case
// where some args must be passed to the test binary.
func findPkgArgPosition(args []string) int {
	if i := boolArgIndex("args", args); i >= 0 {
		return i
	}
	return len(args)
}

type proc struct {
	cmd    waiter
	stdout io.Reader
	stderr io.Reader
	// signal is atomically set to the signal value when a signal is received
	// by newSignalHandler.
	signal int32
}

type waiter interface {
	Wait() error
}

func startGoTest(ctx context.Context, dir string, args []string) (*proc, error) {
	if len(args) == 0 {
		return nil, errors.New("missing command to run")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Dir = dir

	p := proc{cmd: cmd}
	log.Debugf("exec: %s", cmd.Args)
	var err error
	p.stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	p.stderr, err = cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run %s: %w", strings.Join(cmd.Args, " "), err)
	}
	log.Debugf("go test pid: %d", cmd.Process.Pid)

	ctx, cancel := context.WithCancel(ctx)
	newSignalHandler(ctx, cmd.Process.Pid, &p)
	p.cmd = &cancelWaiter{cancel: cancel, wrapped: p.cmd}
	return &p, nil
}

// ExitCodeWithDefault returns the ExitStatus of a process from the error returned by
// exec.Run(). If the exit status is not available an error is returned.
func ExitCodeWithDefault(err error) int {
	if err == nil {
		return 0
	}
	if exiterr, ok := err.(exitCoder); ok {
		if code := exiterr.ExitCode(); code != -1 {
			return code
		}
	}
	return 127
}

type exitCoder interface {
	ExitCode() int
}

func IsExitCoder(err error) bool {
	_, ok := err.(exitCoder)
	return ok
}

type exitError struct {
	num int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit code %d", e.num)
}

func (e exitError) ExitCode() int {
	return e.num
}

// signalExitCode is the base value added to a signal number to produce the
// exit code value. This matches the behaviour of bash.
const signalExitCode = 128

func newSignalHandler(ctx context.Context, pid int, p *proc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		defer signal.Stop(c)

		select {
		case <-ctx.Done():
			return
		case s := <-c:
			atomic.StoreInt32(&p.signal, int32(s.(syscall.Signal)))

			proc, err := os.FindProcess(pid)
			if err != nil {
				log.Errorf("failed to find pid of 'go test': %v", err)
				return
			}
			if err := proc.Signal(s); err != nil {
				log.Errorf("failed to interrupt 'go test': %v", err)
				return
			}
		}
	}()
}

// cancelWaiter wraps a waiter to cancel the context after the wrapped
// Wait exits.
type cancelWaiter struct {
	cancel  func()
	wrapped waiter
}

func (w *cancelWaiter) Wait() error {
	err := w.wrapped.Wait()
	w.cancel()
	return err
}
