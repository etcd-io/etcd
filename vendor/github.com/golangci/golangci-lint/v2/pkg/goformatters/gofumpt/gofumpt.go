package gofumpt

import (
	"strings"

	gofumpt "mvdan.cc/gofumpt/format"

	"github.com/golangci/golangci-lint/v2/pkg/config"
)

const Name = "gofumpt"

type Formatter struct {
	options gofumpt.Options
}

func New(settings *config.GoFumptSettings, goVersion string) *Formatter {
	var options gofumpt.Options

	if settings != nil {
		options = gofumpt.Options{
			LangVersion: getLangVersion(goVersion),
			ModulePath:  settings.ModulePath,
			ExtraRules:  settings.ExtraRules,
		}
	}

	return &Formatter{options: options}
}

func (*Formatter) Name() string {
	return Name
}

func (f *Formatter) Format(_ string, src []byte) ([]byte, error) {
	return gofumpt.Source(src, f.options)
}

func getLangVersion(v string) string {
	return "go" + strings.TrimPrefix(v, "go")
}
