// Package compose provides composition of the linter components.
package compose

import (
	"os"

	"github.com/godoc-lint/godoc-lint/pkg/analysis"
	"github.com/godoc-lint/godoc-lint/pkg/check"
	"github.com/godoc-lint/godoc-lint/pkg/config"
	"github.com/godoc-lint/godoc-lint/pkg/inspect"
	"github.com/godoc-lint/godoc-lint/pkg/model"
)

// Composition holds the composed components of the linter.
type Composition struct {
	Registry      model.Registry
	ConfigBuilder model.ConfigBuilder
	Inspector     model.Inspector
	Analyzer      model.Analyzer
}

// CompositionConfig holds the configuration for composing the linter.
type CompositionConfig struct {
	BaseDir  string
	ExitFunc func(int, error)

	// BaseDirPlainConfig holds the plain configuration for the base directory.
	//
	// This is meant to be used for integrating with umbrella linters (e.g.
	// golangci-lint) where the root config comes from a different source/format.
	BaseDirPlainConfig *config.PlainConfig
}

// Compose composes the linter components based on the given configuration.
func Compose(c CompositionConfig) *Composition {
	if c.BaseDir == "" {
		// It's a best effort to use the current working directory if not set.
		c.BaseDir, _ = os.Getwd()
	}

	reg := check.NewPopulatedRegistry()
	cb := config.NewConfigBuilder(c.BaseDir).WithBaseDirPlainConfig(c.BaseDirPlainConfig)
	ocb := config.NewOnceConfigBuilder(cb)
	inspector := inspect.NewInspector(ocb, c.ExitFunc)
	analyzer := analysis.NewAnalyzer(c.BaseDir, ocb, reg, inspector, c.ExitFunc)

	return &Composition{
		Registry:      reg,
		ConfigBuilder: cb,
		Inspector:     inspector,
		Analyzer:      analyzer,
	}
}
