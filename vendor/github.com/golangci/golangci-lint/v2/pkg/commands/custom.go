package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/golangci/golangci-lint/v2/pkg/commands/internal"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

const envKeepTempFiles = "CUSTOM_GCL_KEEP_TEMP_FILES"

type customOptions struct {
	version     string
	name        string
	destination string
}

type customCommand struct {
	cmd *cobra.Command

	cfg *internal.Configuration

	opts customOptions

	log logutils.Log
}

func newCustomCommand(logger logutils.Log) *customCommand {
	c := &customCommand{log: logger}

	customCmd := &cobra.Command{
		Use:          "custom",
		Short:        "Build a version of golangci-lint with custom linters.",
		Args:         cobra.NoArgs,
		PreRunE:      c.preRunE,
		RunE:         c.runE,
		SilenceUsage: true,
	}

	flagSet := customCmd.PersistentFlags()
	flagSet.SortFlags = false // sort them as they are defined here

	flagSet.StringVar(&c.opts.version, "version", "", color.GreenString("The golangci-lint version used to build the custom binary"))
	flagSet.StringVar(&c.opts.name, "name", "", color.GreenString("The name of the custom binary"))
	flagSet.StringVar(&c.opts.destination, "destination", "", color.GreenString("The directory path used to store the custom binary"))

	c.cmd = customCmd

	return c
}

func (c *customCommand) preRunE(_ *cobra.Command, _ []string) error {
	cfg, err := internal.LoadConfiguration()
	if err != nil {
		return err
	}

	if c.opts.version != "" {
		cfg.Version = c.opts.version
	}

	if c.opts.name != "" {
		cfg.Name = c.opts.name
	}

	if c.opts.destination != "" {
		cfg.Destination = c.opts.destination
	}

	err = cfg.Validate()
	if err != nil {
		return err
	}

	c.cfg = cfg

	return nil
}

func (c *customCommand) runE(cmd *cobra.Command, _ []string) error {
	tmp, err := os.MkdirTemp(os.TempDir(), "custom-gcl")
	if err != nil {
		return fmt.Errorf("create temporary directory: %w", err)
	}

	defer func() {
		if os.Getenv(envKeepTempFiles) != "" {
			log.Printf("WARN: The env var %s has been detected: the temporary directory is preserved: %s", envKeepTempFiles, tmp)

			return
		}

		_ = os.RemoveAll(tmp)
	}()

	err = internal.NewBuilder(c.log, c.cfg, tmp).Build(cmd.Context())
	if err != nil {
		return fmt.Errorf("build process: %w", err)
	}

	return nil
}
