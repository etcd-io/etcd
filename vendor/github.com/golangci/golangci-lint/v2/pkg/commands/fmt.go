package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goformat"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result/processors"
)

type fmtOptions struct {
	config.LoaderOptions

	diff        bool // Flag only.
	diffColored bool // Flag only.
	stdin       bool // Flag only.
}

type fmtCommand struct {
	viper *viper.Viper
	cmd   *cobra.Command

	opts fmtOptions

	cfg *config.Config

	buildInfo BuildInfo

	runner *goformat.Runner

	log logutils.Log
}

func newFmtCommand(logger logutils.Log, info BuildInfo) *fmtCommand {
	c := &fmtCommand{
		viper:     viper.New(),
		log:       logger,
		cfg:       config.NewDefault(),
		buildInfo: info,
	}

	fmtCmd := &cobra.Command{
		Use:               "fmt",
		Short:             "Format Go source files.",
		RunE:              c.execute,
		PreRunE:           c.preRunE,
		PersistentPreRunE: c.persistentPreRunE,
		PersistentPostRun: c.persistentPostRun,
		SilenceUsage:      true,
	}

	fmtCmd.SetOut(logutils.StdOut) // use custom output to properly color it in Windows terminals
	fmtCmd.SetErr(logutils.StdErr)

	fs := fmtCmd.Flags()
	fs.SortFlags = false // sort them as they are defined here

	setupConfigFileFlagSet(fs, &c.opts.LoaderOptions)

	setupFormattersFlagSet(c.viper, fs)

	fs.BoolVarP(&c.opts.diff, "diff", "d", false, color.GreenString("Display diffs instead of rewriting files"))
	fs.BoolVar(&c.opts.diffColored, "diff-colored", false, color.GreenString("Display diffs instead of rewriting files (with colors)"))
	fs.BoolVar(&c.opts.stdin, "stdin", false, color.GreenString("Use standard input for piping source files"))

	c.cmd = fmtCmd

	return c
}

func (c *fmtCommand) persistentPreRunE(cmd *cobra.Command, args []string) error {
	c.log.Infof("%s", c.buildInfo.String())

	loader := config.NewFormattersLoader(c.log.Child(logutils.DebugKeyConfigReader), c.viper, cmd.Flags(), c.opts.LoaderOptions, c.cfg, args)

	err := loader.Load(config.LoadOptions{CheckDeprecation: true, Validation: true})
	if err != nil {
		return fmt.Errorf("can't load config: %w", err)
	}

	return nil
}

func (c *fmtCommand) preRunE(_ *cobra.Command, _ []string) error {
	if c.cfg.GetConfigDir() != "" && c.cfg.Version != "2" {
		return fmt.Errorf("invalid version of the configuration: %q", c.cfg.Version)
	}

	metaFormatter, err := goformatters.NewMetaFormatter(c.log, &c.cfg.Formatters, &c.cfg.Run)
	if err != nil {
		return fmt.Errorf("failed to create meta-formatter: %w", err)
	}

	matcher := processors.NewGeneratedFileMatcher(c.cfg.Formatters.Exclusions.Generated)

	opts, err := goformat.NewRunnerOptions(c.cfg, c.opts.diff, c.opts.diffColored, c.opts.stdin)
	if err != nil {
		return fmt.Errorf("build walk options: %w", err)
	}

	c.runner = goformat.NewRunner(c.log, metaFormatter, matcher, opts)

	return nil
}

func (c *fmtCommand) execute(_ *cobra.Command, args []string) error {
	paths := cleanArgs(args)

	c.log.Infof("Formatting Go files...")

	err := c.runner.Run(paths)
	if err != nil {
		return fmt.Errorf("failed to process files: %w", err)
	}

	return nil
}

func (c *fmtCommand) persistentPostRun(_ *cobra.Command, _ []string) {
	if c.runner.ExitCode() != 0 {
		os.Exit(c.runner.ExitCode())
	}
}

func cleanArgs(args []string) []string {
	if len(args) == 0 {
		return []string{"."}
	}

	var expanded []string
	for _, arg := range args {
		expanded = append(expanded, filepath.Clean(strings.ReplaceAll(arg, "...", "")))
	}

	return expanded
}
