package gci

import (
	"context"
	"go/format"

	gcicfg "github.com/daixiang0/gci/pkg/config"
	"github.com/daixiang0/gci/pkg/gci"
	"github.com/daixiang0/gci/pkg/log"
	"github.com/ldez/grignotin/gomod"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	gcicfgi "github.com/golangci/golangci-lint/v2/pkg/goformatters/gci/internal/config"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/internal"
)

const Name = "gci"

type Formatter struct {
	config *gcicfg.Config
}

func New(settings *config.GciSettings) (*Formatter, error) {
	log.InitLogger()
	_ = log.L().Sync()

	modPath, err := gomod.GetModulePath(context.Background())
	if err != nil {
		internal.FormatterLogger.Errorf("gci: %v", err)
	}

	cfg := gcicfgi.YamlConfig{
		Cfg: gcicfg.BoolConfig{
			NoInlineComments: settings.NoInlineComments,
			NoPrefixComments: settings.NoPrefixComments,
			CustomOrder:      settings.CustomOrder,
			NoLexOrder:       settings.NoLexOrder,

			// Should be managed with `formatters.exclusions.generated`.
			SkipGenerated: false,
		},
		SectionStrings: settings.Sections,
		ModPath:        modPath,
	}

	parsedCfg, err := cfg.Parse()
	if err != nil {
		return nil, err
	}

	return &Formatter{config: &gcicfg.Config{
		BoolConfig:        parsedCfg.BoolConfig,
		Sections:          parsedCfg.Sections,
		SectionSeparators: parsedCfg.SectionSeparators,
	}}, nil
}

func (*Formatter) Name() string {
	return Name
}

func (f *Formatter) Format(filename string, src []byte) ([]byte, error) {
	_, formatted, err := gci.LoadFormat(src, filename, *f.config)
	if err != nil {
		return nil, err
	}

	// gci format the code only when the imports are modified,
	// this produced inconsistencies.
	// To be always consistent, the code should always be formatted.
	// https://github.com/daixiang0/gci/blob/c4f689991095c0e54843dca76fb9c3bad58ec5c7/pkg/gci/gci.go#L148-L151
	// https://github.com/daixiang0/gci/blob/c4f689991095c0e54843dca76fb9c3bad58ec5c7/pkg/gci/gci.go#L215
	return format.Source(formatted)
}
