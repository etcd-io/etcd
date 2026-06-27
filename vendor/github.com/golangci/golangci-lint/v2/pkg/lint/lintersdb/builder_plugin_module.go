package lintersdb

import (
	"fmt"
	"strings"

	"github.com/golangci/plugin-module-register/register"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

const modulePluginType = "module"

// PluginModuleBuilder builds the custom linters (module plugin) based on the configuration.
type PluginModuleBuilder struct {
	log logutils.Log
}

// NewPluginModuleBuilder creates new PluginModuleBuilder.
func NewPluginModuleBuilder(log logutils.Log) *PluginModuleBuilder {
	return &PluginModuleBuilder{log: log}
}

// Build loads custom linters that are specified in the golangci-lint config file.
func (b *PluginModuleBuilder) Build(cfg *config.Config) ([]*linter.Config, error) {
	if cfg == nil || b.log == nil {
		return nil, nil
	}

	var linters []*linter.Config

	for name, settings := range cfg.Linters.Settings.Custom {
		if settings.Type != modulePluginType {
			continue
		}

		b.log.Infof("Loaded %s: %s", settings.Path, name)

		newPlugin, err := register.GetPlugin(name)
		if err != nil {
			return nil, fmt.Errorf("plugin(%s): %w", name, err)
		}

		p, err := newPlugin(settings.Settings)
		if err != nil {
			return nil, fmt.Errorf("plugin(%s): newPlugin %w", name, err)
		}

		analyzers, err := p.BuildAnalyzers()
		if err != nil {
			return nil, fmt.Errorf("plugin(%s): BuildAnalyzers %w", name, err)
		}

		customLinter := goanalysis.NewLinter(name, settings.Description, analyzers, nil)

		switch strings.ToLower(p.GetLoadMode()) {
		case register.LoadModeSyntax:
			customLinter = customLinter.WithLoadMode(goanalysis.LoadModeSyntax)
		case register.LoadModeTypesInfo:
			customLinter = customLinter.WithLoadMode(goanalysis.LoadModeTypesInfo)
		default:
			customLinter = customLinter.WithLoadMode(goanalysis.LoadModeTypesInfo)
		}

		lc := linter.NewConfig(customLinter).
			WithGroups(config.GroupStandard).
			WithURL(settings.OriginalURL)

		switch strings.ToLower(p.GetLoadMode()) {
		case register.LoadModeSyntax:
			// noop
		case register.LoadModeTypesInfo:
			lc = lc.WithLoadForGoAnalysis()
		default:
			lc = lc.WithLoadForGoAnalysis()
		}

		linters = append(linters, lc)
	}

	return linters, nil
}
