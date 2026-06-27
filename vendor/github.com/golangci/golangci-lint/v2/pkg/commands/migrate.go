package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/fatih/color"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	jsonsch "github.com/golangci/golangci-lint/v2/jsonschema"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/fakeloader"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/parser"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versionone"
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/exitcodes"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type migrateOptions struct {
	config.LoaderOptions

	format         string // Flag only.
	skipValidation bool   // Flag only.
}
type migrateCommand struct {
	viper *viper.Viper
	cmd   *cobra.Command

	opts migrateOptions

	cfg *versionone.Config

	buildInfo BuildInfo

	log logutils.Log
}

func newMigrateCommand(log logutils.Log, info BuildInfo) *migrateCommand {
	c := &migrateCommand{
		viper:     viper.New(),
		cfg:       versionone.NewConfig(),
		buildInfo: info,
		log:       log,
	}

	migrateCmd := &cobra.Command{
		Use:               "migrate",
		Short:             "Migrate configuration file from v1 to v2.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		Args:              cobra.NoArgs,
		RunE:              c.execute,
		PreRunE:           c.preRunE,
		PersistentPreRunE: c.persistentPreRunE,
	}

	migrateCmd.SetOut(logutils.StdOut) // use custom output to properly color it in Windows terminals
	migrateCmd.SetErr(logutils.StdErr)

	fs := migrateCmd.Flags()
	fs.SortFlags = false // sort them as they are defined here

	setupConfigFileFlagSet(fs, &c.opts.LoaderOptions)

	fs.StringVar(&c.opts.format, "format", "",
		color.GreenString("Output file format.\nBy default, the format of the input configuration file is used.\n"+
			"It can be 'yml', 'yaml', 'toml', or 'json'."))

	fs.BoolVar(&c.opts.skipValidation, "skip-validation", false,
		color.GreenString("Skip validation of the configuration file against the JSON Schema for v1."))

	c.cmd = migrateCmd

	return c
}

func (c *migrateCommand) execute(_ *cobra.Command, _ []string) error {
	srcPath := c.viper.ConfigFileUsed()
	if srcPath == "" {
		c.log.Warnf("No config file detected")
		os.Exit(exitcodes.NoConfigFileDetected)
	}

	err := c.backupConfigurationFile(srcPath)
	if err != nil {
		return err
	}

	c.log.Warnf("The configuration comments are not migrated.")
	c.log.Warnf("Details about the migration: https://golangci-lint.run/docs/product/migration-guide/")

	c.log.Infof("Migrating v1 configuration file: %s", srcPath)

	ext := filepath.Ext(srcPath)

	if c.opts.format != "" {
		ext = "." + c.opts.format
	}

	if !strings.EqualFold(filepath.Ext(srcPath), ext) {
		defer func() {
			_ = os.RemoveAll(srcPath)
		}()
	}

	if c.cfg.Run.Timeout != 0 {
		c.log.Warnf("The configuration `run.timeout` is ignored. By default, in v2, the timeout is disabled.")
	}

	newCfg := migrate.ToConfig(c.cfg)

	dstPath := strings.TrimSuffix(srcPath, filepath.Ext(srcPath)) + ext

	err = saveNewConfiguration(newCfg, dstPath)
	if err != nil {
		return fmt.Errorf("saving configuration file: %w", err)
	}

	c.log.Infof("Migration done: %s", dstPath)

	callForAction(c.cmd)

	return nil
}

func (c *migrateCommand) preRunE(cmd *cobra.Command, _ []string) error {
	switch strings.ToLower(c.opts.format) {
	case "", "yml", "yaml", "toml", "json":
		// Valid format.
	default:
		return fmt.Errorf("unsupported format: %s", c.opts.format)
	}

	if c.cfg.Version != "" {
		return fmt.Errorf("configuration version is already set: %s", c.cfg.Version)
	}

	if c.opts.skipValidation {
		return nil
	}

	usedConfigFile := c.viper.ConfigFileUsed()
	if usedConfigFile == "" {
		c.log.Warnf("No config file detected")
		os.Exit(exitcodes.NoConfigFileDetected)
	}

	c.log.Infof("Validating v1 configuration file: %s", usedConfigFile)

	err := validateConfiguration(jsonsch.V1Schema, usedConfigFile)
	if err != nil {
		var v *jsonschema.ValidationError
		if !errors.As(err, &v) {
			return fmt.Errorf("[%s] validate: %w", usedConfigFile, err)
		}

		printValidationDetail(cmd, v.DetailedOutput())

		return errors.New("the configuration contains invalid elements")
	}

	return nil
}

func (c *migrateCommand) persistentPreRunE(_ *cobra.Command, args []string) error {
	c.log.Infof("%s", c.buildInfo.String())

	loader := config.NewBaseLoader(c.log.Child(logutils.DebugKeyConfigReader), c.viper, c.opts.LoaderOptions, fakeloader.NewConfig(), args)

	// Loads the configuration just to get the effective path of the configuration.
	err := loader.Load()
	if err != nil {
		return fmt.Errorf("can't load config: %w", err)
	}

	srcPath := c.viper.ConfigFileUsed()
	if srcPath == "" {
		c.log.Warnf("No config file detected")
		os.Exit(exitcodes.NoConfigFileDetected)
	}

	return fakeloader.Load(srcPath, c.cfg)
}

func (c *migrateCommand) backupConfigurationFile(srcPath string) error {
	filename := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath)) + ".bck" + filepath.Ext(srcPath)
	dstPath := filepath.Join(filepath.Dir(srcPath), filename)

	c.log.Infof("Saving the v1 configuration to: %s", dstPath)

	stat, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	err = os.WriteFile(dstPath, data, stat.Mode())
	if err != nil {
		return err
	}

	return nil
}

func saveNewConfiguration(cfg any, dstPath string) error {
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	defer func() { _ = dstFile.Close() }()

	return parser.Encode(cfg, dstFile)
}

func callForAction(cmd *cobra.Command) {
	pStyle := lipgloss.NewStyle().
		Padding(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("161")).
		Align(lipgloss.Center)

	hStyle := lipgloss.NewStyle().Bold(true)

	s := fmt.Sprintln(hStyle.Render("We need you!"))
	s += `
Donations help fund the ongoing development and maintenance of this tool.
If golangci-lint has been useful to you, please consider contributing.

Donate now: https://donate.golangci.org`

	cmd.Println(pStyle.Render(s))
}
