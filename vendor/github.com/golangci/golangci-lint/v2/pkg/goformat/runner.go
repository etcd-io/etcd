package goformat

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	rpdiff "github.com/rogpeppe/go-internal/diff"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result/processors"
)

type Runner struct {
	log logutils.Log

	metaFormatter *goformatters.MetaFormatter
	matcher       *processors.GeneratedFileMatcher

	opts RunnerOptions

	exitCode int
}

func NewRunner(logger logutils.Log,
	metaFormatter *goformatters.MetaFormatter, matcher *processors.GeneratedFileMatcher,
	opts RunnerOptions) *Runner {
	return &Runner{
		log:           logger,
		matcher:       matcher,
		metaFormatter: metaFormatter,
		opts:          opts,
	}
}

func (c *Runner) Run(paths []string) error {
	savedStdout, savedStderr := os.Stdout, os.Stderr

	if !logutils.HaveDebugTag(logutils.DebugKeyFormattersOutput) {
		// Don't allow linters and loader to print anything
		log.SetOutput(io.Discard)
		c.setOutputToDevNull()
		defer func() {
			os.Stdout, os.Stderr = savedStdout, savedStderr
		}()
	}

	if c.opts.stdin {
		return c.formatStdIn("<standard input>", savedStdout, os.Stdin)
	}

	for _, path := range paths {
		err := c.walk(path, savedStdout)
		if err != nil {
			return err
		}
	}

	for pattern, count := range c.opts.excludedPathCounter {
		if c.opts.warnUnused && count == 0 {
			c.log.Warnf("The pattern %q match no issues", pattern)
		} else {
			c.log.Infof("Skipped %d issues by pattern %q", count, pattern)
		}
	}

	return nil
}

func (c *Runner) walk(root string, stdout *os.File) error {
	return filepath.Walk(root, func(path string, f fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() && skipDir(f.Name()) {
			return fs.SkipDir
		}

		if !isGoFile(f) {
			return nil
		}

		match, err := c.opts.MatchAnyPattern(path)
		if err != nil || match {
			return err
		}

		//nolint:gosec // See explanation below.
		// `path` contains the `root` but when using `r, err := os.OpenRoot(root)`, this part is not inside the file tree of `r`.
		// `filepath.Rel()` can be used but it seems overkill in the context and doesn't work well with a file.
		in, err := os.Open(path)
		if err != nil {
			return err
		}

		defer func() { _ = in.Close() }()

		return c.process(path, stdout, in)
	})
}

func (c *Runner) process(path string, stdout io.Writer, in io.Reader) error {
	input, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	match, err := c.matcher.IsGeneratedFile(path, input)
	if err != nil || match {
		return err
	}

	output := c.metaFormatter.Format(path, input)

	if bytes.Equal(input, output) {
		return nil
	}

	if c.opts.diff {
		newName := filepath.ToSlash(path)
		oldName := newName + ".orig"

		patch := rpdiff.Diff(oldName, input, newName, output)

		if c.opts.colors {
			err = quick.Highlight(stdout, string(patch), "diff", "terminal", "native")
			if err != nil {
				return err
			}
		} else {
			_, err = stdout.Write(patch)
			if err != nil {
				return err
			}
		}

		c.exitCode = 1

		return nil
	}

	c.log.Infof("format: %s", path)

	// On Windows, we need to re-set the permissions from the file. See golang/go#38225.
	var perms os.FileMode
	if fi, err := os.Stat(path); err == nil {
		perms = fi.Mode() & os.ModePerm
	}

	return os.WriteFile(path, output, perms)
}

func (c *Runner) formatStdIn(path string, stdout io.Writer, in io.Reader) error {
	input, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	match, err := c.matcher.IsGeneratedFile(path, input)
	if err != nil {
		return err
	}

	if match {
		// If the file is generated,
		// the input should be written to the stdout to avoid emptied the file.
		_, err = stdout.Write(input)
		if err != nil {
			return err
		}

		return nil
	}

	output := c.metaFormatter.Format(path, input)

	_, err = stdout.Write(output)
	if err != nil {
		return err
	}

	return nil
}

func (c *Runner) setOutputToDevNull() {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		c.log.Warnf("Can't open null device %q: %s", os.DevNull, err)
		return
	}

	os.Stdout, os.Stderr = devNull, devNull
}

func (c *Runner) ExitCode() int {
	return c.exitCode
}

type RunnerOptions struct {
	basePath  string
	patterns  []*regexp.Regexp
	generated string
	diff      bool
	colors    bool
	stdin     bool

	warnUnused          bool
	excludedPathCounter map[*regexp.Regexp]int
}

func NewRunnerOptions(cfg *config.Config, diff, diffColored, stdin bool) (RunnerOptions, error) {
	basePath, err := fsutils.GetBasePath(context.Background(), cfg.Run.RelativePathMode, cfg.GetConfigDir())
	if err != nil {
		return RunnerOptions{}, fmt.Errorf("get base path: %w", err)
	}

	// Required to be consistent with `RunnerOptions.MatchAnyPattern`.
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return RunnerOptions{}, err
	}

	opts := RunnerOptions{
		basePath:            absBasePath,
		generated:           cfg.Formatters.Exclusions.Generated,
		diff:                diff || diffColored,
		colors:              diffColored,
		stdin:               stdin,
		excludedPathCounter: make(map[*regexp.Regexp]int),
		warnUnused:          cfg.Formatters.Exclusions.WarnUnused,
	}

	for _, pattern := range cfg.Formatters.Exclusions.Paths {
		exp, err := regexp.Compile(fsutils.NormalizePathInRegex(pattern))
		if err != nil {
			return RunnerOptions{}, fmt.Errorf("compile path pattern %q: %w", pattern, err)
		}

		opts.patterns = append(opts.patterns, exp)
		opts.excludedPathCounter[exp] = 0
	}

	return opts, nil
}

func (o RunnerOptions) MatchAnyPattern(path string) (bool, error) {
	if len(o.patterns) == 0 {
		return false, nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}

	rel, err := filepath.Rel(o.basePath, abs)
	if err != nil {
		return false, err
	}

	for _, pattern := range o.patterns {
		if pattern.MatchString(rel) {
			o.excludedPathCounter[pattern]++
			return true, nil
		}
	}

	return false, nil
}

func skipDir(name string) bool {
	switch name {
	case "vendor", "testdata", "node_modules":
		return true

	default:
		return strings.HasPrefix(name, ".") && name != "."
	}
}

func isGoFile(f fs.FileInfo) bool {
	return !f.IsDir() && !strings.HasPrefix(f.Name(), ".") && strings.HasSuffix(f.Name(), ".go")
}
