package lintersdb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"plugin"

	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

const goPluginType = "goplugin"

type AnalyzerPlugin interface {
	GetAnalyzers() []*analysis.Analyzer
}

// PluginGoBuilder builds the custom linters (Go plugin) based on the configuration.
type PluginGoBuilder struct {
	log logutils.Log
}

// NewPluginGoBuilder creates new PluginGoBuilder.
func NewPluginGoBuilder(log logutils.Log) *PluginGoBuilder {
	return &PluginGoBuilder{log: log}
}

// Build loads custom linters that are specified in the golangci-lint config file.
func (b *PluginGoBuilder) Build(cfg *config.Config) ([]*linter.Config, error) {
	if cfg == nil || b.log == nil {
		return nil, nil
	}

	var linters []*linter.Config

	for name, settings := range cfg.Linters.Settings.Custom {
		if settings.Type != goPluginType && settings.Type != "" {
			continue
		}

		lc, err := b.loadConfig(cfg, name, &settings)
		if err != nil {
			return nil, fmt.Errorf("unable to load custom analyzer %q: %s, %w", name, settings.Path, err)
		}
		linters = append(linters, lc)
	}

	return linters, nil
}

// loadConfig loads the configuration of private linters.
// Private linters are dynamically loaded from .so plugin files.
func (b *PluginGoBuilder) loadConfig(cfg *config.Config, name string, settings *config.CustomLinterSettings) (*linter.Config, error) {
	analyzers, err := b.getAnalyzerPlugin(cfg, settings.Path, settings.Settings)
	if err != nil {
		return nil, err
	}

	b.log.Infof("Loaded %s: %s", settings.Path, name)

	customLinter := goanalysis.NewLinter(name, settings.Description, analyzers, nil).
		WithLoadMode(goanalysis.LoadModeTypesInfo)

	linterConfig := linter.NewConfig(customLinter).
		WithGroups(config.GroupStandard).
		WithLoadForGoAnalysis().
		WithURL(settings.OriginalURL)

	return linterConfig, nil
}

// getAnalyzerPlugin loads a private linter as specified in the config file,
// loads the plugin from a .so file,
// and returns the 'AnalyzerPlugin' interface implemented by the private plugin.
// An error is returned if the private linter cannot be loaded
// or the linter does not implement the AnalyzerPlugin interface.
func (b *PluginGoBuilder) getAnalyzerPlugin(cfg *config.Config, path string, settings any) ([]*analysis.Analyzer, error) {
	if !filepath.IsAbs(path) {
		basePath, err := fsutils.GetBasePath(context.Background(), cfg.Run.RelativePathMode, cfg.GetConfigDir())
		if err != nil {
			return nil, fmt.Errorf("get base path: %w", err)
		}

		// resolve non-absolute paths relative to config file's directory
		path = filepath.Join(basePath, path)
	}

	plug, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}

	analyzers, err := b.lookupPlugin(plug, settings)
	if err != nil {
		return nil, fmt.Errorf("lookup plugin %s: %w", path, err)
	}

	return analyzers, nil
}

func (b *PluginGoBuilder) lookupPlugin(plug *plugin.Plugin, settings any) ([]*analysis.Analyzer, error) {
	symbol, err := plug.Lookup("New")
	if err != nil {
		analyzers, errP := b.lookupAnalyzerPlugin(plug)
		if errP != nil {
			return nil, errors.Join(err, errP)
		}

		return analyzers, nil
	}

	// The type func cannot be used here, must be the explicit signature.
	constructor, ok := symbol.(func(any) ([]*analysis.Analyzer, error))
	if !ok {
		return nil, fmt.Errorf("plugin does not abide by 'New' function: %T", symbol)
	}

	return constructor(settings)
}

func (b *PluginGoBuilder) lookupAnalyzerPlugin(plug *plugin.Plugin) ([]*analysis.Analyzer, error) {
	symbol, err := plug.Lookup("AnalyzerPlugin")
	if err != nil {
		return nil, err
	}

	b.log.Warnf("plugin: 'AnalyzerPlugin' plugins are deprecated, please use the new plugin signature: " +
		"https://golangci-lint.run/docs/plugins/go-plugins#create-a-plugin")

	analyzerPlugin, ok := symbol.(AnalyzerPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin does not abide by 'AnalyzerPlugin' interface: %T", symbol)
	}

	return analyzerPlugin.GetAnalyzers(), nil
}
