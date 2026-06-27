package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
)

type Output struct {
	Formats    Formats  `mapstructure:"formats"`
	SortOrder  []string `mapstructure:"sort-order"`
	ShowStats  bool     `mapstructure:"show-stats"`
	PathPrefix string   `mapstructure:"path-prefix"`
	PathMode   string   `mapstructure:"path-mode"`
}

func (o *Output) Validate() error {
	validators := []func() error{
		o.validateSortOrder,
		o.validatePathMode,
	}

	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}

	return nil
}

func (o *Output) validateSortOrder() error {
	validOrders := []string{"linter", "file", "severity"}

	all := strings.Join(o.SortOrder, " ")

	for _, order := range o.SortOrder {
		if strings.Count(all, order) > 1 {
			return fmt.Errorf("the sort-order name %q is repeated several times", order)
		}

		if !slices.Contains(validOrders, order) {
			return fmt.Errorf("unsupported sort-order name %q", order)
		}
	}

	return nil
}

func (o *Output) validatePathMode() error {
	switch o.PathMode {
	case "", fsutils.OutputPathModeAbsolute:
		// Valid

	default:
		return fmt.Errorf("unsupported output path mode %q", o.PathMode)
	}

	return nil
}
