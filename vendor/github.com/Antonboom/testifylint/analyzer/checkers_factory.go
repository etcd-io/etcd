package analyzer

import (
	"fmt"

	"github.com/Antonboom/testifylint/internal/checkers"
	"github.com/Antonboom/testifylint/internal/config"
)

// newCheckers accepts linter config and returns slices of enabled checkers sorted by priority.
func newCheckers(cfg config.Config) ([]checkers.RegularChecker, []checkers.AdvancedChecker, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	enabledCheckersSet := make(map[string]struct{})

	if cfg.EnableAll {
		for _, checker := range checkers.All() {
			enabledCheckersSet[checker] = struct{}{}
		}
	} else if !cfg.DisableAll {
		for _, checker := range checkers.EnabledByDefault() {
			enabledCheckersSet[checker] = struct{}{}
		}
	}

	for _, checker := range cfg.EnabledCheckers {
		enabledCheckersSet[checker] = struct{}{}
	}

	for _, checker := range cfg.DisabledCheckers {
		delete(enabledCheckersSet, checker)
	}

	enabledCheckers := make([]string, 0, len(enabledCheckersSet))
	for v := range enabledCheckersSet {
		enabledCheckers = append(enabledCheckers, v)
	}
	checkers.SortByPriority(enabledCheckers)

	regularCheckers := make([]checkers.RegularChecker, 0, len(enabledCheckers))
	advancedCheckers := make([]checkers.AdvancedChecker, 0, len(enabledCheckers)/2)

	for _, name := range enabledCheckers {
		ch, ok := checkers.Get(name)
		if !ok {
			return nil, nil, fmt.Errorf("unknown checker %q", name)
		}

		switch c := ch.(type) {
		case *checkers.BoolCompare:
			c.SetIgnoreCustomTypes(cfg.BoolCompare.IgnoreCustomTypes)

		case *checkers.ExpectedActual:
			c.SetExpVarPattern(cfg.ExpectedActual.ExpVarPattern.Regexp)

		case *checkers.Formatter:
			c.SetCheckFormatString(cfg.Formatter.CheckFormatString)
			c.SetRequireFFuncs(cfg.Formatter.RequireFFuncs)
			c.SetRequireStringMsg(cfg.Formatter.RequireStringMsg)

		case *checkers.GoRequire:
			c.SetIgnoreHTTPHandlers(cfg.GoRequire.IgnoreHTTPHandlers)

		case *checkers.RequireError:
			c.SetFnPattern(cfg.RequireError.FnPattern.Regexp)

		case *checkers.SuiteExtraAssertCall:
			c.SetMode(cfg.SuiteExtraAssertCall.Mode)
		}

		switch casted := ch.(type) {
		case checkers.RegularChecker:
			regularCheckers = append(regularCheckers, casted)
		case checkers.AdvancedChecker:
			advancedCheckers = append(advancedCheckers, casted)
		}
	}

	return regularCheckers, advancedCheckers, nil
}
