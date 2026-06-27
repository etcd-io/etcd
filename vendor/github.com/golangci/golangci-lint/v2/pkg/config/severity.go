package config

import (
	"errors"
	"fmt"
)

const severityRuleMinConditionsCount = 1

type Severity struct {
	Default string         `mapstructure:"default"`
	Rules   []SeverityRule `mapstructure:"rules"`
}

func (s *Severity) Validate() error {
	if len(s.Rules) > 0 && s.Default == "" {
		return errors.New("can't set severity rule option: no default severity defined")
	}

	for i, rule := range s.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("error in severity rule #%d: %w", i, err)
		}
	}

	return nil
}

type SeverityRule struct {
	BaseRule `mapstructure:",squash"`
	Severity string `mapstructure:"severity"`
}

func (s *SeverityRule) Validate() error {
	if s.Severity == "" {
		return errors.New("severity should be set")
	}

	return s.BaseRule.Validate(severityRuleMinConditionsCount)
}
