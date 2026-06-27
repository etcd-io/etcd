package lintersdb

import (
	"fmt"
	"os"
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type Validator struct {
	m *Manager
}

func NewValidator(m *Manager) *Validator {
	return &Validator{m: m}
}

// Validate validates the configuration by calling all other validators for different
// sections in the configuration and then some additional linter validation functions.
func (v Validator) Validate(cfg *config.Config) error {
	validators := []func(cfg *config.Linters) error{
		v.validateLintersNames,
		v.alternativeNamesDeprecation,
	}

	for _, v := range validators {
		if err := v(&cfg.Linters); err != nil {
			return err
		}
	}

	return nil
}

func (v Validator) validateLintersNames(cfg *config.Linters) error {
	var unknownNames []string

	for _, name := range cfg.Enable {
		if v.m.GetLinterConfigs(name) == nil {
			unknownNames = append(unknownNames, name)
		}
	}

	for _, name := range cfg.Disable {
		lcs := v.m.GetLinterConfigs(name)
		if len(lcs) == 0 {
			unknownNames = append(unknownNames, name)
			continue
		}

		for _, lc := range lcs {
			if lc.IsDeprecated() && lc.Deprecation.Level > linter.DeprecationWarning {
				v.m.log.Warnf("The linter %q is deprecated (step 2) and deactivated. "+
					"It should be removed from the list of disabled linters. "+
					"https://golangci-lint.run/docs/product/roadmap/#linter-deprecation-cycle", lc.Name())
			}
		}
	}

	if len(unknownNames) > 0 {
		return fmt.Errorf("unknown linters: '%v', run 'golangci-lint help linters' to see the list of supported linters",
			strings.Join(unknownNames, ","))
	}

	return nil
}

func (v Validator) alternativeNamesDeprecation(cfg *config.Linters) error {
	if v.m.cfg.InternalTest || v.m.cfg.InternalCmdTest || os.Getenv(logutils.EnvTestRun) == "1" {
		return nil
	}

	altNames := map[string][]string{}
	for _, lc := range v.m.GetAllSupportedLinterConfigs() {
		for _, alt := range lc.AlternativeNames {
			altNames[alt] = append(altNames[alt], lc.Name())
		}
	}

	names := cfg.Enable
	names = append(names, cfg.Disable...)

	for _, name := range names {
		lc, ok := altNames[name]
		if !ok {
			continue
		}

		if len(lc) > 1 {
			v.m.log.Warnf("The linter named %q is deprecated. It has been split into: %s.", name, strings.Join(lc, ", "))
		} else {
			v.m.log.Warnf("The name %q is deprecated. The linter has been renamed to: %s.", name, lc[0])
		}
	}

	return nil
}
