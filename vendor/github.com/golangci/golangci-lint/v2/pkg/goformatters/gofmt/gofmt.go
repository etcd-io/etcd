package gofmt

import (
	"github.com/golangci/gofmt/gofmt"

	"github.com/golangci/golangci-lint/v2/pkg/config"
)

const Name = "gofmt"

type Formatter struct {
	options gofmt.Options
}

func New(settings *config.GoFmtSettings) *Formatter {
	options := gofmt.Options{}

	if settings != nil {
		options.NeedSimplify = settings.Simplify

		for _, rule := range settings.RewriteRules {
			options.RewriteRules = append(options.RewriteRules, gofmt.RewriteRule(rule))
		}
	}

	return &Formatter{options: options}
}

func (*Formatter) Name() string {
	return Name
}

func (f *Formatter) Format(filename string, src []byte) ([]byte, error) {
	return gofmt.Source(filename, src, f.options)
}
