package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/goutil"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

var errConfigDisabled = errors.New("config is disabled by --no-config")

const (
	modeLinters    = "linters"
	modeFormatters = "formatters"
)

type LoaderOptions struct {
	Config   string // Flag only. The path to the golangci config file, as specified with the --config argument.
	NoConfig bool   // Flag only.
}

type LoadOptions struct {
	CheckDeprecation bool
	Validation       bool
}

type Loader struct {
	*BaseLoader

	fs *pflag.FlagSet

	cfg *Config

	mode string
}

func NewLintersLoader(log logutils.Log, v *viper.Viper, fs *pflag.FlagSet, opts LoaderOptions, cfg *Config, args []string) *Loader {
	loader := newLoader(log, v, fs, opts, cfg, args)
	loader.mode = modeLinters

	return loader
}

func NewFormattersLoader(log logutils.Log, v *viper.Viper, fs *pflag.FlagSet, opts LoaderOptions, cfg *Config, args []string) *Loader {
	loader := newLoader(log, v, fs, opts, cfg, args)
	loader.mode = modeFormatters

	return loader
}

func newLoader(log logutils.Log, v *viper.Viper, fs *pflag.FlagSet, opts LoaderOptions, cfg *Config, args []string) *Loader {
	return &Loader{
		BaseLoader: NewBaseLoader(log, v, opts, cfg, args),
		fs:         fs,
		cfg:        cfg,
	}
}

func (l *Loader) Load(opts LoadOptions) error {
	err := l.BaseLoader.Load()
	if err != nil {
		return err
	}

	if l.mode == modeLinters {
		l.applyStringSliceHack()
	}

	if l.cfg.Linters.Exclusions.Generated == "" {
		l.cfg.Linters.Exclusions.Generated = GeneratedModeStrict
	}

	err = l.checkConfigurationVersion()
	if err != nil {
		return err
	}

	if !l.cfg.InternalCmdTest {
		for _, n := range slices.Concat(l.cfg.Linters.Enable, l.cfg.Linters.Disable) {
			if n == "typecheck" {
				return fmt.Errorf("%s is not a linter, it cannot be enabled or disabled", n)
			}
		}
	}

	l.handleFormatters()

	if opts.CheckDeprecation {
		err = l.handleDeprecation()
		if err != nil {
			return err
		}

		l.handleFormatterDeprecations()
	}

	l.handleGoVersion()

	err = goutil.CheckGoVersion(l.cfg.Run.Go)
	if err != nil {
		return err
	}

	l.cfg.basePath, err = fsutils.GetBasePath(context.Background(), l.cfg.Run.RelativePathMode, l.cfg.cfgDir)
	if err != nil {
		return fmt.Errorf("get base path: %w", err)
	}

	err = l.handleEnableOnlyOption()
	if err != nil {
		return err
	}

	if opts.Validation {
		err = l.cfg.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// Hack to append values from StringSlice flags.
// Viper always overrides StringSlice values.
// https://github.com/spf13/viper/issues/1448
// So StringSlice flags are not bind to Viper like that their values are obtain via Cobra Flags.
func (l *Loader) applyStringSliceHack() {
	if l.fs == nil {
		return
	}

	l.appendStringSlice("enable", &l.cfg.Linters.Enable)
	l.appendStringSlice("disable", &l.cfg.Linters.Disable)
	l.appendStringSlice("build-tags", &l.cfg.Run.BuildTags)
}

func (l *Loader) appendStringSlice(name string, current *[]string) {
	if l.fs.Changed(name) {
		val, _ := l.fs.GetStringSlice(name)
		*current = append(*current, val...)
	}
}

func (l *Loader) checkConfigurationVersion() error {
	if l.cfg.GetConfigDir() != "" && l.cfg.Version != "2" {
		return fmt.Errorf("unsupported version of the configuration: %q "+
			"See https://golangci-lint.run/docs/product/migration-guide for migration instructions", l.cfg.Version)
	}

	return nil
}

func (l *Loader) handleGoVersion() {
	if l.cfg.Run.Go == "" {
		l.cfg.Run.Go = detectGoVersion(context.Background(), l.log)
	}

	l.cfg.Linters.Settings.Govet.Go = l.cfg.Run.Go

	l.cfg.Linters.Settings.ParallelTest.Go = l.cfg.Run.Go

	l.cfg.Linters.Settings.GoFumpt.LangVersion = l.cfg.Run.Go
	l.cfg.Formatters.Settings.GoFumpt.LangVersion = l.cfg.Run.Go

	trimmedGoVersion := goutil.TrimGoVersion(l.cfg.Run.Go)

	l.cfg.Linters.Settings.Revive.Go = trimmedGoVersion

	l.cfg.Linters.Settings.Gocritic.Go = trimmedGoVersion

	os.Setenv("GOSECGOVERSION", l.cfg.Run.Go)
}

func (l *Loader) handleDeprecation() error {
	if l.cfg.InternalTest || l.cfg.InternalCmdTest || os.Getenv(logutils.EnvTestRun) == "1" {
		return nil
	}

	l.handleLinterOptionDeprecations()

	return nil
}

func (l *Loader) handleLinterOptionDeprecations() {
	// Deprecated since v2.1.0.
	if l.cfg.Linters.Settings.Goconst.IgnoreStrings != "" {
		l.log.Warnf("The configuration option `linters.settings.goconst.ignore-strings` is deprecated, " +
			"please use `linters.settings.goconst.ignore-string-values`.")

		l.cfg.Linters.Settings.Goconst.IgnoreStringValues = append(l.cfg.Linters.Settings.Goconst.IgnoreStringValues,
			l.cfg.Linters.Settings.Goconst.IgnoreStrings)
	}
}

func (l *Loader) handleEnableOnlyOption() error {
	lookup := l.fs.Lookup("enable-only")
	if lookup == nil {
		return nil
	}

	only, err := l.fs.GetStringSlice("enable-only")
	if err != nil {
		return err
	}

	if len(only) > 0 {
		l.cfg.Linters = Linters{
			Default:    GroupNone,
			Enable:     only,
			Settings:   l.cfg.Linters.Settings,
			Exclusions: l.cfg.Linters.Exclusions,
		}

		l.cfg.Formatters = Formatters{}
	}

	return nil
}

func (l *Loader) handleFormatters() {
	l.handleFormatterOverrides()
	l.handleFormatterExclusions()
}

// Overrides linter settings with formatter settings if the formatter is enabled.
func (l *Loader) handleFormatterOverrides() {
	if slices.Contains(l.cfg.Formatters.Enable, "gofmt") {
		l.cfg.Linters.Settings.GoFmt = l.cfg.Formatters.Settings.GoFmt
	}

	if slices.Contains(l.cfg.Formatters.Enable, "gofumpt") {
		l.cfg.Linters.Settings.GoFumpt = l.cfg.Formatters.Settings.GoFumpt
	}

	if slices.Contains(l.cfg.Formatters.Enable, "goimports") {
		l.cfg.Linters.Settings.GoImports = l.cfg.Formatters.Settings.GoImports
	}

	if slices.Contains(l.cfg.Formatters.Enable, "gci") {
		l.cfg.Linters.Settings.Gci = l.cfg.Formatters.Settings.Gci
	}

	if slices.Contains(l.cfg.Formatters.Enable, "golines") {
		l.cfg.Linters.Settings.GoLines = l.cfg.Formatters.Settings.GoLines
	}
}

// Add formatter exclusions to linters exclusions.
func (l *Loader) handleFormatterExclusions() {
	if len(l.cfg.Formatters.Enable) == 0 {
		return
	}

	for _, path := range l.cfg.Formatters.Exclusions.Paths {
		l.cfg.Linters.Exclusions.Rules = append(l.cfg.Linters.Exclusions.Rules, ExcludeRule{
			BaseRule: BaseRule{
				Linters: l.cfg.Formatters.Enable,
				Path:    path,
			},
		})
	}
}

func (*Loader) handleFormatterDeprecations() {
	// The function is empty but deprecations will happen in the future.
}
