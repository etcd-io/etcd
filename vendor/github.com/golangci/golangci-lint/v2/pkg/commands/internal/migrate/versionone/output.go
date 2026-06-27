package versionone

import (
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/ptr"
)

type Output struct {
	Formats         OutputFormats `mapstructure:"formats"`
	PrintIssuedLine *bool         `mapstructure:"print-issued-lines"`
	PrintLinterName *bool         `mapstructure:"print-linter-name"`
	SortResults     *bool         `mapstructure:"sort-results"`
	SortOrder       []string      `mapstructure:"sort-order"`
	PathPrefix      *string       `mapstructure:"path-prefix"`
	ShowStats       *bool         `mapstructure:"show-stats"`
}

type OutputFormat struct {
	Format *string `mapstructure:"format"`
	Path   *string `mapstructure:"path"`
}

type OutputFormats []OutputFormat

func (p *OutputFormats) UnmarshalText(text []byte) error {
	for item := range strings.SplitSeq(string(text), ",") {
		format, path, _ := strings.Cut(item, ":")

		*p = append(*p, OutputFormat{
			Path:   ptr.Pointer(path),
			Format: ptr.Pointer(format),
		})
	}

	return nil
}
