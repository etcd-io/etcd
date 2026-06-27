package goimports

import (
	"strings"

	"golang.org/x/tools/imports"

	"github.com/golangci/golangci-lint/v2/pkg/config"
)

const Name = "goimports"

type Formatter struct{}

func New(settings *config.GoImportsSettings) *Formatter {
	if settings != nil {
		imports.LocalPrefix = strings.Join(settings.LocalPrefixes, ",")
	}

	return &Formatter{}
}

func (*Formatter) Name() string {
	return Name
}

func (*Formatter) Format(filename string, src []byte) ([]byte, error) {
	// The `imports.LocalPrefix` (`settings.LocalPrefixes`) is a global var.
	return imports.Process(filename, src, nil)
}
