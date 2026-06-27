package analyzer

import (
	"flag"
	"fmt"

	"golang.org/x/tools/go/analysis"
)

type ConfigurableModeType string

const (
	ConfigurableAnalyzerDeferOnly ConfigurableModeType = "defer-only"
	ConfigurableAnalyzerClosed    ConfigurableModeType = "closed"
)

type ConifgurableAnalyzer struct {
	Mode string
}

func NewConfigurableAnalyzer(mode ConfigurableModeType) *analysis.Analyzer {
	cfgAnalyzer := &ConifgurableAnalyzer{}
	flags := flag.NewFlagSet("cfgAnalyzer", flag.ExitOnError)
	flags.StringVar(&cfgAnalyzer.Mode, "mode", string(mode),
		"Mode to run the analyzer in. (defer-only, closed)")
	return newAnalyzer(cfgAnalyzer.run, flags)
}

func (c *ConifgurableAnalyzer) run(pass *analysis.Pass) (interface{}, error) {
	switch c.Mode {
	case string(ConfigurableAnalyzerDeferOnly):
		analyzer := &deferOnlyAnalyzer{}
		return analyzer.Run(pass)
	case string(ConfigurableAnalyzerClosed):
		analyzer := &closedAnalyzer{}
		return analyzer.Run(pass)
	default:
		return nil, fmt.Errorf("invalid mode: %s", c.Mode)
	}
}
