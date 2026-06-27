// Package lintcmd implements the frontend of an analysis runner.
// It serves as the entry-point for the staticcheck command, and can also be used to implement custom linters that behave like staticcheck.
package lintcmd

import (
	"bufio"
	"encoding/gob"
	"flag"
	"fmt"
	"go/token"
	stdversion "go/version"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/config"
	"honnef.co/go/tools/go/loader"
	"honnef.co/go/tools/lintcmd/version"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/buildutil"
)

type buildConfig struct {
	Name  string
	Envs  []string
	Flags []string
}

// Command represents a linter command line tool.
type Command struct {
	name           string
	analyzers      map[string]*lint.Analyzer
	version        string
	machineVersion string

	flags struct {
		fs *flag.FlagSet

		tags        string
		tests       bool
		showIgnored bool
		formatter   string

		// mutually exclusive mode flags
		explain      string
		printVersion bool
		listChecks   bool
		merge        bool

		matrix bool

		debugCpuprofile       string
		debugMemprofile       string
		debugVersion          bool
		debugNoCompileErrors  bool
		debugMeasureAnalyzers string
		debugTrace            string

		checks    list
		fail      list
		goVersion versionFlag
	}
}

// NewCommand returns a new Command.
func NewCommand(name string) *Command {
	cmd := &Command{
		name:           name,
		analyzers:      map[string]*lint.Analyzer{},
		version:        "devel",
		machineVersion: "devel",
	}
	cmd.initFlagSet(name)
	return cmd
}

// SetVersion sets the command's version.
// It is divided into a human part and a machine part.
// For example, Staticcheck 2020.2.1 had the human version "2020.2.1" and the machine version "v0.1.1".
// If you only use Semver, you can set both parts to the same value.
//
// Calling this method is optional. Both versions default to "devel", and we'll attempt to deduce more version information from the Go module.
func (cmd *Command) SetVersion(human, machine string) {
	cmd.version = human
	cmd.machineVersion = machine
}

// FlagSet returns the command's flag set.
// This can be used to add additional command line arguments.
func (cmd *Command) FlagSet() *flag.FlagSet {
	return cmd.flags.fs
}

// AddAnalyzers adds analyzers to the command.
// These are lint.Analyzer analyzers, which wrap analysis.Analyzer analyzers, bundling them with structured documentation.
//
// To add analysis.Analyzer analyzers without providing structured documentation, use AddBareAnalyzers.
func (cmd *Command) AddAnalyzers(as ...*lint.Analyzer) {
	for _, a := range as {
		cmd.analyzers[a.Analyzer.Name] = a
	}
}

// AddBareAnalyzers adds bare analyzers to the command.
func (cmd *Command) AddBareAnalyzers(as ...*analysis.Analyzer) {
	for _, a := range as {
		var title, text string
		if idx := strings.Index(a.Doc, "\n\n"); idx > -1 {
			title = a.Doc[:idx]
			text = a.Doc[idx+2:]
		}

		doc := &lint.RawDocumentation{
			Title:    title,
			Text:     text,
			Severity: lint.SeverityWarning,
		}

		cmd.analyzers[a.Name] = &lint.Analyzer{
			Doc:      doc,
			Analyzer: a,
		}
	}
}

func (cmd *Command) initFlagSet(name string) {
	flags := flag.NewFlagSet("", flag.ExitOnError)
	cmd.flags.fs = flags
	flags.Usage = usage(name, flags)

	flags.StringVar(&cmd.flags.tags, "tags", "", "List of `build tags`")
	flags.BoolVar(&cmd.flags.tests, "tests", true, "Include tests")
	flags.BoolVar(&cmd.flags.printVersion, "version", false, "Print version and exit")
	flags.BoolVar(&cmd.flags.showIgnored, "show-ignored", false, "Don't filter ignored diagnostics")
	flags.StringVar(&cmd.flags.formatter, "f", "text", "Output `format` (valid choices are 'stylish', 'text' and 'json')")
	flags.StringVar(&cmd.flags.explain, "explain", "", "Print description of `check`")
	flags.BoolVar(&cmd.flags.listChecks, "list-checks", false, "List all available checks")
	flags.BoolVar(&cmd.flags.merge, "merge", false, "Merge results of multiple Staticcheck runs")
	flags.BoolVar(&cmd.flags.matrix, "matrix", false, "Read a build config matrix from stdin")

	flags.StringVar(&cmd.flags.debugCpuprofile, "debug.cpuprofile", "", "Write CPU profile to `file`")
	flags.StringVar(&cmd.flags.debugMemprofile, "debug.memprofile", "", "Write memory profile to `file`")
	flags.BoolVar(&cmd.flags.debugVersion, "debug.version", false, "Print detailed version information about this program")
	flags.BoolVar(&cmd.flags.debugNoCompileErrors, "debug.no-compile-errors", false, "Don't print compile errors")
	flags.StringVar(&cmd.flags.debugMeasureAnalyzers, "debug.measure-analyzers", "", "Write analysis measurements to `file`. `file` will be opened for appending if it already exists.")
	flags.StringVar(&cmd.flags.debugTrace, "debug.trace", "", "Write trace to `file`")

	cmd.flags.checks = list{"inherit"}
	cmd.flags.fail = list{"all"}
	cmd.flags.goVersion = versionFlag("module")
	flags.Var(&cmd.flags.checks, "checks", "Comma-separated list of `checks` to enable.")
	flags.Var(&cmd.flags.fail, "fail", "Comma-separated list of `checks` that can cause a non-zero exit status.")
	flags.Var(&cmd.flags.goVersion, "go", "Target Go `version` in the format '1.x', or the literal 'module' to use the module's Go version")
}

type list []string

func (list *list) String() string {
	return `"` + strings.Join(*list, ",") + `"`
}

func (list *list) Set(s string) error {
	if s == "" {
		*list = nil
		return nil
	}

	elems := strings.Split(s, ",")
	for i, elem := range elems {
		elems[i] = strings.TrimSpace(elem)
	}
	*list = elems
	return nil
}

type versionFlag string

func (v *versionFlag) String() string {
	return fmt.Sprintf("%q", string(*v))
}

func (v *versionFlag) Set(s string) error {
	if s == "module" {
		*v = "module"
	} else {
		orig := s
		if !strings.HasPrefix(s, "go") {
			s = "go" + s
		}
		if stdversion.IsValid(s) {
			*v = versionFlag(s)
		} else {
			return fmt.Errorf("%q is not a valid Go version", orig)
		}
	}
	return nil
}

// ParseFlags parses command line flags.
// It must be called before calling Run.
// After calling ParseFlags, the values of flags can be accessed.
//
// Example:
//
//	cmd.ParseFlags(os.Args[1:])
func (cmd *Command) ParseFlags(args []string) {
	cmd.flags.fs.Parse(args)
}

// diagnosticDescriptor represents the uniquely identifying information of diagnostics.
type diagnosticDescriptor struct {
	Position token.Position
	End      token.Position
	Category string
	Message  string
}

func (diag diagnostic) descriptor() diagnosticDescriptor {
	return diagnosticDescriptor{
		Position: diag.Position,
		End:      diag.End,
		Category: diag.Category,
		Message:  diag.Message,
	}
}

type run struct {
	checkedFiles map[string]struct{}
	diagnostics  map[diagnosticDescriptor]diagnostic
}

func runFromLintResult(res lintResult) run {
	out := run{
		checkedFiles: map[string]struct{}{},
		diagnostics:  map[diagnosticDescriptor]diagnostic{},
	}

	for _, cf := range res.CheckedFiles {
		out.checkedFiles[cf] = struct{}{}
	}
	for _, diag := range res.Diagnostics {
		out.diagnostics[diag.descriptor()] = diag
	}
	return out
}

func decodeGob(br io.ByteReader) ([]run, error) {
	var runs []run
	for {
		var res lintResult
		if err := gob.NewDecoder(br.(io.Reader)).Decode(&res); err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}
		runs = append(runs, runFromLintResult(res))
	}
	return runs, nil
}

// Execute runs all registered analyzers and reports their findings.
// The status code returned can be used for os.Exit(cmd.Execute()).
func (cmd *Command) Execute() int {
	// Set up profiling and tracing
	if path := cmd.flags.debugCpuprofile; path != "" {
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
	}
	if path := cmd.flags.debugTrace; path != "" {
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		trace.Start(f)
	}

	// Update the default config's list of enabled checks
	defaultChecks := []string{"all"}
	for _, a := range cmd.analyzers {
		if a.Doc.NonDefault {
			defaultChecks = append(defaultChecks, "-"+a.Analyzer.Name)
		}
	}
	config.DefaultConfig.Checks = defaultChecks

	// Run the appropriate mode
	var exit int
	switch {
	case cmd.flags.debugVersion:
		exit = cmd.printDebugVersion()
	case cmd.flags.listChecks:
		exit = cmd.listChecks()
	case cmd.flags.printVersion:
		exit = cmd.printVersion()
	case cmd.flags.explain != "":
		exit = cmd.explain()
	case cmd.flags.merge:
		exit = cmd.merge()
	default:
		exit = cmd.lint()
	}

	// Stop profiling
	if cmd.flags.debugCpuprofile != "" {
		pprof.StopCPUProfile()
	}
	if path := cmd.flags.debugMemprofile; path != "" {
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		runtime.GC()
		pprof.WriteHeapProfile(f)
	}
	if cmd.flags.debugTrace != "" {
		trace.Stop()
	}

	return exit
}

// Run runs all registered analyzers and reports their findings.
// It always calls os.Exit and does not return.
func (cmd *Command) Run() {
	os.Exit(cmd.Execute())
}

func (cmd *Command) printDebugVersion() int {
	version.Verbose(cmd.version, cmd.machineVersion)
	return 0
}

func (cmd *Command) listChecks() int {
	cs := slices.Collect(maps.Values(cmd.analyzers))
	sort.Slice(cs, func(i, j int) bool {
		return cs[i].Analyzer.Name < cs[j].Analyzer.Name
	})
	for _, c := range cs {
		var title string
		if c.Doc != nil {
			title = c.Doc.Compile().Title
		}
		fmt.Printf("%s %s\n", c.Analyzer.Name, title)
	}
	return 0
}

func (cmd *Command) printVersion() int {
	version.Print(cmd.version, cmd.machineVersion)
	return 0
}

func (cmd *Command) explain() int {
	explain := cmd.flags.explain
	check, ok := cmd.analyzers[explain]
	if !ok {
		fmt.Fprintln(os.Stderr, "Couldn't find check", explain)
		return 1
	}
	if check.Analyzer.Doc == "" {
		fmt.Fprintln(os.Stderr, explain, "has no documentation")
		return 1
	}
	fmt.Println(check.Doc.Compile())
	fmt.Println("Online documentation\n    https://staticcheck.dev/docs/checks#" + check.Analyzer.Name)
	return 0
}

func (cmd *Command) merge() int {
	var runs []run
	if len(cmd.flags.fs.Args()) == 0 {
		var err error
		runs, err = decodeGob(bufio.NewReader(os.Stdin))
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Errorf("couldn't parse stdin: %s", err))
			return 1
		}
	} else {
		for _, path := range cmd.flags.fs.Args() {
			someRuns, err := func(path string) ([]run, error) {
				f, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				br := bufio.NewReader(f)
				return decodeGob(br)
			}(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("couldn't parse file %s: %s", path, err))
				return 1
			}
			runs = append(runs, someRuns...)
		}
	}

	relevantDiagnostics := mergeRuns(runs)
	cs := slices.Collect(maps.Values(cmd.analyzers))
	return cmd.printDiagnostics(cs, relevantDiagnostics)
}

func (cmd *Command) lint() int {
	switch cmd.flags.formatter {
	case "text", "stylish", "json", "sarif", "binary", "null":
	default:
		fmt.Fprintf(os.Stderr, "unsupported output format %q\n", cmd.flags.formatter)
		return 2
	}

	var bconfs []buildConfig
	if cmd.flags.matrix {
		if cmd.flags.tags != "" {
			fmt.Fprintln(os.Stderr, "cannot use -matrix and -tags together")
			return 2
		}

		var err error
		bconfs, err = parseBuildConfigs(os.Stdin)
		if err != nil {
			if perr, ok := err.(parseBuildConfigError); ok {
				fmt.Fprintf(os.Stderr, "<stdin>:%d couldn't parse build matrix: %s\n", perr.line, perr.err)
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
			return 2
		}
	} else {
		bc := buildConfig{}
		if cmd.flags.tags != "" {
			// Validate that the tags argument is well-formed. go/packages
			// doesn't detect malformed build flags and returns unhelpful
			// errors.
			tf := buildutil.TagsFlag{}
			if err := tf.Set(cmd.flags.tags); err != nil {
				fmt.Fprintln(os.Stderr, fmt.Errorf("invalid value %q for flag -tags: %s", cmd.flags.tags, err))
				return 1
			}

			bc.Flags = []string{"-tags", cmd.flags.tags}
		}
		bconfs = append(bconfs, bc)
	}

	var measureAnalyzers func(analysis *analysis.Analyzer, pkg *loader.PackageSpec, d time.Duration)
	if path := cmd.flags.debugMeasureAnalyzers; path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}

		mu := &sync.Mutex{}
		measureAnalyzers = func(analysis *analysis.Analyzer, pkg *loader.PackageSpec, d time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			// FIXME(dh): print pkg.ID
			if _, err := fmt.Fprintf(f, "%s\t%s\t%d\n", analysis.Name, pkg, d.Nanoseconds()); err != nil {
				log.Println("error writing analysis measurements:", err)
			}
		}
	}

	var runs []run
	cs := slices.Collect(maps.Values(cmd.analyzers))
	opts := options{
		analyzers: cs,
		patterns:  cmd.flags.fs.Args(),
		lintTests: cmd.flags.tests,
		goVersion: string(cmd.flags.goVersion),
		config: config.Config{
			Checks: cmd.flags.checks,
		},
		printAnalyzerMeasurement: measureAnalyzers,
	}
	l, err := newLinter(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, bconf := range bconfs {
		res, err := l.run(bconf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		for _, w := range res.Warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}

		cwd, err := os.Getwd()
		if err != nil {
			cwd = ""
		}
		relPath := func(s string) string {
			if cwd == "" {
				return filepath.ToSlash(s)
			}
			out, err := filepath.Rel(cwd, s)
			if err != nil {
				return filepath.ToSlash(s)
			}
			return filepath.ToSlash(out)
		}

		if cmd.flags.formatter == "binary" {
			for i, s := range res.CheckedFiles {
				res.CheckedFiles[i] = relPath(s)
			}
			for i := range res.Diagnostics {
				// We turn all paths into relative, /-separated paths. This is to make -merge work correctly when
				// merging runs from different OSs, with different absolute paths.
				//
				// We zero out Offset, because checkouts of code on different OSs may have different kinds of
				// newlines and thus different offsets. We don't ever make use of the Offset, anyway. Line and
				// column numbers are precomputed.

				d := &res.Diagnostics[i]
				d.Position.Filename = relPath(d.Position.Filename)
				d.Position.Offset = 0
				d.End.Filename = relPath(d.End.Filename)
				d.End.Offset = 0
				for j := range d.Related {
					r := &d.Related[j]
					r.Position.Filename = relPath(r.Position.Filename)
					r.Position.Offset = 0
					r.End.Filename = relPath(r.End.Filename)
					r.End.Offset = 0
				}
			}
			err := gob.NewEncoder(os.Stdout).Encode(res)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed writing output: %s\n", err)
				return 2
			}
		} else {
			runs = append(runs, runFromLintResult(res))
		}
	}

	l.cache.Trim()

	if cmd.flags.formatter != "binary" {
		diags := mergeRuns(runs)
		return cmd.printDiagnostics(cs, diags)
	}
	return 0
}

func mergeRuns(runs []run) []diagnostic {
	var relevantDiagnostics []diagnostic
	for _, r := range runs {
		for _, diag := range r.diagnostics {
			switch diag.MergeIf {
			case lint.MergeIfAny:
				relevantDiagnostics = append(relevantDiagnostics, diag)
			case lint.MergeIfAll:
				doPrint := true
				for _, r := range runs {
					if _, ok := r.checkedFiles[diag.Position.Filename]; ok {
						if _, ok := r.diagnostics[diag.descriptor()]; !ok {
							doPrint = false
						}
					}
				}
				if doPrint {
					relevantDiagnostics = append(relevantDiagnostics, diag)
				}
			}
		}
	}
	return relevantDiagnostics
}

// printDiagnostics prints the diagnostics and exits the process.
func (cmd *Command) printDiagnostics(cs []*lint.Analyzer, diagnostics []diagnostic) int {
	if len(diagnostics) > 1 {
		sort.Slice(diagnostics, func(i, j int) bool {
			di := diagnostics[i]
			dj := diagnostics[j]
			pi := di.Position
			pj := dj.Position

			if pi.Filename != pj.Filename {
				return pi.Filename < pj.Filename
			}
			if pi.Line != pj.Line {
				return pi.Line < pj.Line
			}
			if pi.Column != pj.Column {
				return pi.Column < pj.Column
			}
			if di.Message != dj.Message {
				return di.Message < dj.Message
			}
			if di.BuildName != dj.BuildName {
				return di.BuildName < dj.BuildName
			}
			return di.Category < dj.Category
		})

		filtered := []diagnostic{
			diagnostics[0],
		}
		builds := []map[string]struct{}{
			{diagnostics[0].BuildName: {}},
		}
		for _, diag := range diagnostics[1:] {
			// We may encounter duplicate diagnostics because one file
			// can be part of many packages, and because multiple
			// build configurations may check the same files.
			if !filtered[len(filtered)-1].equal(diag) {
				if filtered[len(filtered)-1].descriptor() == diag.descriptor() {
					// Diagnostics only differ in build name, track new name
					builds[len(filtered)-1][diag.BuildName] = struct{}{}
				} else {
					filtered = append(filtered, diag)
					builds = append(builds, map[string]struct{}{})
					builds[len(filtered)-1][diag.BuildName] = struct{}{}
				}
			}
		}

		var names []string
		for i := range filtered {
			names = names[:0]
			for k := range builds[i] {
				names = append(names, k)
			}
			sort.Strings(names)
			filtered[i].BuildName = strings.Join(names, ",")
		}
		diagnostics = filtered
	}

	var f formatter
	switch cmd.flags.formatter {
	case "text":
		f = textFormatter{W: os.Stdout}
	case "stylish":
		f = &stylishFormatter{W: os.Stdout}
	case "json":
		f = jsonFormatter{W: os.Stdout}
	case "sarif":
		f = &sarifFormatter{
			driverName:    cmd.name,
			driverVersion: cmd.version,
		}
		if cmd.name == "staticcheck" {
			f.(*sarifFormatter).driverName = "Staticcheck"
			f.(*sarifFormatter).driverWebsite = "https://staticcheck.dev"
		}
	case "binary":
		fmt.Fprintln(os.Stderr, "'-f binary' not supported in this context")
		return 2
	case "null":
		f = nullFormatter{}
	default:
		fmt.Fprintf(os.Stderr, "unsupported output format %q\n", cmd.flags.formatter)
		return 2
	}

	fail := cmd.flags.fail
	analyzerNames := make([]string, len(cs))
	for i, a := range cs {
		analyzerNames[i] = a.Analyzer.Name
	}
	shouldExit := filterAnalyzerNames(analyzerNames, fail)
	shouldExit["staticcheck"] = true
	shouldExit["compile"] = true
	shouldExit["config"] = true

	var (
		numErrors   int
		numWarnings int
		numIgnored  int
	)
	notIgnored := make([]diagnostic, 0, len(diagnostics))
	for _, diag := range diagnostics {
		if diag.Category == "compile" && cmd.flags.debugNoCompileErrors {
			continue
		}
		if diag.Severity == severityIgnored && !cmd.flags.showIgnored {
			numIgnored++
			continue
		}
		if shouldExit[diag.Category] {
			numErrors++
		} else {
			diag.Severity = severityWarning
			numWarnings++
		}
		notIgnored = append(notIgnored, diag)
	}

	f.Format(cs, notIgnored)
	if f, ok := f.(statter); ok {
		f.Stats(len(diagnostics), numErrors, numWarnings, numIgnored)
	}

	if numErrors > 0 {
		if _, ok := f.(*sarifFormatter); ok {
			// When emitting SARIF, finding errors is considered success.
			return 0
		} else {
			return 1
		}
	}
	return 0
}

func usage(name string, fs *flag.FlagSet) func() {
	return func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [packages]\n", name)

		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		printDefaults(fs)

		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "For help about specifying packages, see 'go help packages'")
	}
}

// isZeroValue determines whether the string represents the zero
// value for a flag.
//
// this function has been copied from the Go standard library's 'flag' package.
func isZeroValue(f *flag.Flag, value string) bool {
	// Build a zero value of the flag's Value type, and see if the
	// result of calling its String method equals the value passed in.
	// This works unless the Value type is itself an interface type.
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Pointer {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	return value == z.Interface().(flag.Value).String()
}

// this function has been copied from the Go standard library's 'flag' package and modified to skip debug flags.
func printDefaults(fs *flag.FlagSet) {
	fs.VisitAll(func(f *flag.Flag) {
		// Don't print debug flags
		if strings.HasPrefix(f.Name, "debug.") {
			return
		}

		var b strings.Builder
		fmt.Fprintf(&b, "  -%s", f.Name) // Two spaces before -; see next two comments.
		name, usage := flag.UnquoteUsage(f)
		if len(name) > 0 {
			b.WriteString(" ")
			b.WriteString(name)
		}
		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if b.Len() <= 4 { // space, space, '-', 'x'.
			b.WriteString("\t")
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			b.WriteString("\n    \t")
		}
		b.WriteString(strings.ReplaceAll(usage, "\n", "\n    \t"))

		if !isZeroValue(f, f.DefValue) {
			if T := reflect.TypeOf(f.Value); T.Name() == "*stringValue" && T.PkgPath() == "flag" {
				// put quotes on the value
				fmt.Fprintf(&b, " (default %q)", f.DefValue)
			} else {
				fmt.Fprintf(&b, " (default %v)", f.DefValue)
			}
		}
		fmt.Fprint(fs.Output(), b.String(), "\n")
	})
}
