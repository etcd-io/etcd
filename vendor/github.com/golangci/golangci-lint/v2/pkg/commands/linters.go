package commands

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/lint/lintersdb"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type lintersHelp struct {
	Enabled  []linterHelp
	Disabled []linterHelp
}

type lintersOptions struct {
	config.LoaderOptions
	JSON bool
}

type lintersCommand struct {
	viper *viper.Viper
	cmd   *cobra.Command

	opts lintersOptions

	cfg *config.Config

	log logutils.Log

	dbManager *lintersdb.Manager
}

func newLintersCommand(logger logutils.Log) *lintersCommand {
	c := &lintersCommand{
		viper: viper.New(),
		cfg:   config.NewDefault(),
		log:   logger,
	}

	lintersCmd := &cobra.Command{
		Use:               "linters",
		Short:             "List current linters configuration.",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              c.execute,
		PreRunE:           c.preRunE,
		SilenceUsage:      true,
	}

	fs := lintersCmd.Flags()
	fs.SortFlags = false // sort them as they are defined here

	setupConfigFileFlagSet(fs, &c.opts.LoaderOptions)
	setupLintersFlagSet(c.viper, fs)

	fs.BoolVar(&c.opts.JSON, "json", false, color.GreenString("Display as JSON"))

	c.cmd = lintersCmd

	return c
}

func (c *lintersCommand) preRunE(cmd *cobra.Command, args []string) error {
	loader := config.NewLintersLoader(c.log.Child(logutils.DebugKeyConfigReader), c.viper, cmd.Flags(), c.opts.LoaderOptions, c.cfg, args)

	err := loader.Load(config.LoadOptions{Validation: true})
	if err != nil {
		return fmt.Errorf("can't load config: %w", err)
	}

	dbManager, err := lintersdb.NewManager(c.log.Child(logutils.DebugKeyLintersDB), c.cfg,
		lintersdb.NewLinterBuilder(), lintersdb.NewPluginModuleBuilder(c.log), lintersdb.NewPluginGoBuilder(c.log))
	if err != nil {
		return err
	}

	c.dbManager = dbManager

	return nil
}

func (c *lintersCommand) execute(_ *cobra.Command, _ []string) error {
	enabledLintersMap, err := c.dbManager.GetEnabledLintersMap()
	if err != nil {
		return fmt.Errorf("can't get enabled linters: %w", err)
	}

	var enabledLinters []*linter.Config
	var disabledLinters []*linter.Config

	for _, lc := range c.dbManager.GetAllSupportedLinterConfigs() {
		if lc.Internal {
			continue
		}

		if goformatters.IsFormatter(lc.Name()) {
			continue
		}

		if enabledLintersMap[lc.Name()] == nil {
			disabledLinters = append(disabledLinters, lc)
		} else {
			enabledLinters = append(enabledLinters, lc)
		}
	}

	if c.opts.JSON {
		linters := lintersHelp{}

		for _, lc := range enabledLinters {
			linters.Enabled = append(linters.Enabled, newLinterHelp(lc))
		}

		for _, lc := range disabledLinters {
			linters.Disabled = append(linters.Disabled, newLinterHelp(lc))
		}

		return json.NewEncoder(c.cmd.OutOrStdout()).Encode(linters)
	}

	color.Green("Enabled by your configuration linters:\n")
	printLinters(enabledLinters)

	color.Red("\nDisabled by your configuration linters:\n")
	printLinters(disabledLinters)

	return nil
}
