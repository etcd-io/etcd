package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/exitcodes"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type pathOptions struct {
	JSON bool
}

type configCommand struct {
	viper *viper.Viper
	cmd   *cobra.Command

	opts     config.LoaderOptions
	pathOpts pathOptions

	buildInfo BuildInfo

	log logutils.Log
}

func newConfigCommand(log logutils.Log, info BuildInfo) *configCommand {
	c := &configCommand{
		viper:     viper.New(),
		log:       log,
		buildInfo: info,
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration file information and verification.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: c.preRunE,
	}

	verifyCommand := &cobra.Command{
		Use:               "verify",
		Short:             "Verify configuration against JSON schema.",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              c.executeVerify,
		SilenceUsage:      true,
		SilenceErrors:     true,
	}

	pathCommand := &cobra.Command{
		Use:               "path",
		Short:             "Print used configuration path.",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              c.executePath,
	}

	configCmd.AddCommand(
		pathCommand,
		verifyCommand,
	)

	flagSet := configCmd.PersistentFlags()
	flagSet.SortFlags = false // sort them as they are defined here

	setupConfigFileFlagSet(flagSet, &c.opts)

	pathFlagSet := pathCommand.Flags()
	pathFlagSet.BoolVar(&c.pathOpts.JSON, "json", false, color.GreenString("Display as JSON"))

	c.cmd = configCmd

	return c
}

func (c *configCommand) preRunE(cmd *cobra.Command, args []string) error {
	// The command doesn't depend on the real configuration.
	// It only needs to know the path of the configuration file.
	cfg := config.NewDefault()

	loader := config.NewLintersLoader(c.log.Child(logutils.DebugKeyConfigReader), c.viper, cmd.Flags(), c.opts, cfg, args)

	err := loader.Load(config.LoadOptions{})
	if err != nil {
		return fmt.Errorf("can't load config: %w", err)
	}

	return nil
}

func (c *configCommand) executePath(cmd *cobra.Command, _ []string) error {
	usedConfigFile := c.getUsedConfig()

	if c.pathOpts.JSON {
		abs, err := filepath.Abs(usedConfigFile)
		if err != nil {
			return err
		}

		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
			"path":         usedConfigFile,
			"absolutePath": abs,
		})
	}

	if usedConfigFile == "" {
		c.log.Warnf("No config file detected")
		os.Exit(exitcodes.NoConfigFileDetected)
	}

	cmd.Println(usedConfigFile)

	return nil
}

// getUsedConfig returns the resolved path to the golangci config file,
// or the empty string if no configuration could be found.
func (c *configCommand) getUsedConfig() string {
	usedConfigFile := c.viper.ConfigFileUsed()
	if usedConfigFile == "" {
		return ""
	}

	prettyUsedConfigFile, err := fsutils.ShortestRelPath(usedConfigFile, "")
	if err != nil {
		c.log.Warnf("Can't pretty print config file path: %s", err)
		return usedConfigFile
	}

	return prettyUsedConfigFile
}
