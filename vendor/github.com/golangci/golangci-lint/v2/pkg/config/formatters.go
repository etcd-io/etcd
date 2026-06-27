package config

import (
	"fmt"
	"slices"
)

type Formatters struct {
	Enable     []string            `mapstructure:"enable"`
	Settings   FormatterSettings   `mapstructure:"settings"`
	Exclusions FormatterExclusions `mapstructure:"exclusions"`
}

func (f *Formatters) Validate() error {
	for _, n := range f.Enable {
		if !slices.Contains(getAllFormatterNames(), n) {
			return fmt.Errorf("%s is not a formatter", n)
		}
	}

	return nil
}

type FormatterExclusions struct {
	Generated  string   `mapstructure:"generated"`
	Paths      []string `mapstructure:"paths"`
	WarnUnused bool     `mapstructure:"warn-unused"`
}
