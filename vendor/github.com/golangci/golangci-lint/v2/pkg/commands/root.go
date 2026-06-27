package commands

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

func Execute(info BuildInfo) error {
	return newRootCommand(info).Execute()
}

type rootOptions struct {
	PrintVersion bool // Flag only.

	Verbose bool   // Flag only.
	Color   string // Flag only.
}

type rootCommand struct {
	cmd  *cobra.Command
	opts rootOptions

	log logutils.Log
}

func newRootCommand(info BuildInfo) *rootCommand {
	c := &rootCommand{}

	rootCmd := &cobra.Command{
		Use:   "golangci-lint",
		Short: "golangci-lint is a smart linters runner.",
		Long:  `Smart, fast linters runner.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c.opts.PrintVersion {
				_ = printVersion(logutils.StdOut, info)
				return nil
			}

			return cmd.Help()
		},
	}

	fs := rootCmd.Flags()
	fs.BoolVar(&c.opts.PrintVersion, "version", false, color.GreenString("Print version"))

	setupRootPersistentFlags(rootCmd.PersistentFlags(), &c.opts)

	log := logutils.NewStderrLog(logutils.DebugKeyEmpty)

	// Each command uses a dedicated configuration structure to avoid side effects of bindings.
	rootCmd.AddCommand(
		newLintersCommand(log).cmd,
		newFormattersCommand(log).cmd,
		newRunCommand(log, info).cmd,
		newFmtCommand(log, info).cmd,
		newMigrateCommand(log, info).cmd,
		newCacheCommand().cmd,
		newConfigCommand(log, info).cmd,
		newVersionCommand(info).cmd,
		newCustomCommand(log).cmd,
	)

	rootCmd.SetHelpCommand(newHelpCommand(log).cmd)

	c.log = log
	c.cmd = rootCmd

	return c
}

func (c *rootCommand) Execute() error {
	err := setupLogger(c.log)
	if err != nil {
		return err
	}

	return c.cmd.Execute()
}

func setupRootPersistentFlags(fs *pflag.FlagSet, opts *rootOptions) {
	fs.BoolP("help", "h", false, color.GreenString("Help for a command"))
	fs.BoolVarP(&opts.Verbose, "verbose", "v", false, color.GreenString("Verbose output"))
	fs.StringVar(&opts.Color, "color", "auto", color.GreenString("Use color when printing; can be 'always', 'auto', or 'never'"))
}

func setupLogger(logger logutils.Log) error {
	opts, err := forceRootParsePersistentFlags()
	if err != nil && !errors.Is(err, pflag.ErrHelp) {
		return err
	}

	if opts == nil {
		return nil
	}

	logutils.SetupVerboseLog(logger, opts.Verbose)

	switch opts.Color {
	case "always":
		color.NoColor = false
	case "never":
		color.NoColor = true
	case "auto":
		// nothing
	default:
		logger.Fatalf("invalid value %q for --color; must be 'always', 'auto', or 'never'", opts.Color)
	}

	// For log level colors (mainly for verbose output)
	logutils.DisableColors(color.NoColor)

	return nil
}

func forceRootParsePersistentFlags() (*rootOptions, error) {
	// We use another pflag.FlagSet here to not set `changed` flag on cmd.Flags() options.
	// Otherwise, string slice options will be duplicated.
	fs := pflag.NewFlagSet("config flag set", pflag.ContinueOnError)

	// Ignore unknown flags because we will parse the command flags later.
	fs.ParseErrorsAllowlist = pflag.ParseErrorsAllowlist{UnknownFlags: true}

	opts := &rootOptions{}

	// Don't do `fs.AddFlagSet(cmd.Flags())` because it shares flags representations:
	// `changed` variable inside string slice vars will be shared.
	// Use another config variable here,
	// to not affect main parsing by this parsing of only config option.
	setupRootPersistentFlags(fs, opts)

	fs.Usage = func() {} // otherwise, help text will be printed twice

	if err := fs.Parse(safeArgs(fs, os.Args)); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil, err
		}

		return nil, fmt.Errorf("can't parse args: %w", err)
	}

	return opts, nil
}

// Shorthands are a problem because pflag, with UnknownFlags, will try to parse all the letters as options.
// A shorthand can aggregate several letters (ex `ps -aux`)
// The function replaces non-supported shorthands by a dumb flag.
func safeArgs(fs *pflag.FlagSet, args []string) []string {
	var shorthands []string
	fs.VisitAll(func(flag *pflag.Flag) {
		shorthands = append(shorthands, flag.Shorthand)
	})

	var cleanArgs []string
	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' && !slices.Contains(shorthands, string(arg[1])) {
			cleanArgs = append(cleanArgs, "--potato")
			continue
		}

		cleanArgs = append(cleanArgs, arg)
	}

	return cleanArgs
}
