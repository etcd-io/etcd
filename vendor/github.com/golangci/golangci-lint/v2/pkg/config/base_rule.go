package config

import (
	"errors"
	"fmt"
	"regexp"
)

type BaseRule struct {
	Linters    []string `mapstructure:"linters"`
	Path       string   `mapstructure:"path"`
	PathExcept string   `mapstructure:"path-except"`
	Text       string   `mapstructure:"text"`
	Source     string   `mapstructure:"source"`

	// For compatibility with exclude-use-default/include.
	InternalReference string `mapstructure:"-"`
}

func (b *BaseRule) Validate(minConditionsCount int) error {
	if err := validateOptionalRegex(b.Path); err != nil {
		return fmt.Errorf("invalid path regex: %w", err)
	}

	if err := validateOptionalRegex(b.PathExcept); err != nil {
		return fmt.Errorf("invalid path-except regex: %w", err)
	}

	if err := validateOptionalRegex(b.Text); err != nil {
		return fmt.Errorf("invalid text regex: %w", err)
	}

	if err := validateOptionalRegex(b.Source); err != nil {
		return fmt.Errorf("invalid source regex: %w", err)
	}

	if b.Path != "" && b.PathExcept != "" {
		return errors.New("path and path-except should not be set at the same time")
	}

	nonBlank := 0
	if len(b.Linters) > 0 {
		nonBlank++
	}

	// Filtering by path counts as one condition, regardless how it is done (one or both).
	// Otherwise, a rule with Path and PathExcept set would pass validation
	// whereas before the introduction of path-except that wouldn't have been precise enough.
	if b.Path != "" || b.PathExcept != "" {
		nonBlank++
	}

	if b.Text != "" {
		nonBlank++
	}

	if b.Source != "" {
		nonBlank++
	}

	if nonBlank < minConditionsCount {
		return fmt.Errorf("at least %d of (text, source, path[-except], linters) should be set", minConditionsCount)
	}

	return nil
}

func validateOptionalRegex(value string) error {
	if value == "" {
		return nil
	}

	_, err := regexp.Compile(value)
	return err
}
