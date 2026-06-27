package golines

import (
	"github.com/golangci/golines/shorten"

	"github.com/golangci/golangci-lint/v2/pkg/config"
)

const Name = "golines"

type Formatter struct {
	shortener *shorten.Shortener
}

func New(settings *config.GoLinesSettings) *Formatter {
	cfg := &shorten.Config{}

	if settings != nil {
		cfg = &shorten.Config{
			MaxLen:          settings.MaxLen,
			TabLen:          settings.TabLen,
			KeepAnnotations: false, // golines debug (not usable inside golangci-lint)
			ShortenComments: settings.ShortenComments,
			ReformatTags:    settings.ReformatTags,
			DotFile:         "", // golines debug (not usable inside golangci-lint)
			ChainSplitDots:  settings.ChainSplitDots,
		}
	}

	return &Formatter{shortener: shorten.NewShortener(cfg)}
}

func (*Formatter) Name() string {
	return Name
}

func (f *Formatter) Format(_ string, src []byte) ([]byte, error) {
	return f.shortener.Process(src)
}
