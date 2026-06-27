package loggercheck

import (
	"errors"
	"fmt"

	"github.com/timonwong/loggercheck/internal/checkers"
	"github.com/timonwong/loggercheck/internal/rules"
)

var (
	staticRuleList = []rules.Ruleset{
		mustNewStaticRuleSet("logr", []string{
			"(github.com/go-logr/logr.Logger).Error",
			"(github.com/go-logr/logr.Logger).Info",
			"(github.com/go-logr/logr.Logger).WithValues",
		}),
		mustNewStaticRuleSet("klog", []string{
			"k8s.io/klog/v2.InfoS",
			"k8s.io/klog/v2.InfoSDepth",
			"k8s.io/klog/v2.ErrorS",
			"(k8s.io/klog/v2.Verbose).InfoS",
			"(k8s.io/klog/v2.Verbose).InfoSDepth",
			"(k8s.io/klog/v2.Verbose).ErrorS",
		}),
		mustNewStaticRuleSet("zap", []string{
			"(*go.uber.org/zap.SugaredLogger).With",
			"(*go.uber.org/zap.SugaredLogger).Debugw",
			"(*go.uber.org/zap.SugaredLogger).Infow",
			"(*go.uber.org/zap.SugaredLogger).Warnw",
			"(*go.uber.org/zap.SugaredLogger).Errorw",
			"(*go.uber.org/zap.SugaredLogger).DPanicw",
			"(*go.uber.org/zap.SugaredLogger).Panicw",
			"(*go.uber.org/zap.SugaredLogger).Fatalw",
		}),
		mustNewStaticRuleSet("kitlog", []string{
			"github.com/go-kit/log.With",
			"github.com/go-kit/log.WithPrefix",
			"github.com/go-kit/log.WithSuffix",
			"(github.com/go-kit/log.Logger).Log",
		}),
		mustNewStaticRuleSet("slog", []string{
			"log/slog.Group",

			"log/slog.With",

			"log/slog.Debug",
			"log/slog.Info",
			"log/slog.Warn",
			"log/slog.Error",

			"log/slog.DebugContext",
			"log/slog.InfoContext",
			"log/slog.WarnContext",
			"log/slog.ErrorContext",

			"(*log/slog.Logger).With",

			"(*log/slog.Logger).Debug",
			"(*log/slog.Logger).Info",
			"(*log/slog.Logger).Warn",
			"(*log/slog.Logger).Error",

			"(*log/slog.Logger).DebugContext",
			"(*log/slog.Logger).InfoContext",
			"(*log/slog.Logger).WarnContext",
			"(*log/slog.Logger).ErrorContext",
		}),
	}
	checkerByRulesetName = map[string]checkers.Checker{
		// by default, checkers.General will be used.
		"zap":  checkers.Zap{},
		"slog": checkers.Slog{},
	}
)

// mustNewStaticRuleSet only called at init, catch errors during development.
// In production it will not panic.
func mustNewStaticRuleSet(name string, lines []string) rules.Ruleset {
	if len(lines) == 0 {
		panic(errors.New("no rules provided"))
	}

	rulesetList, err := rules.ParseRules(lines)
	if err != nil {
		panic(err)
	}

	if len(rulesetList) != 1 {
		panic(fmt.Errorf("expected 1 ruleset, got %d", len(rulesetList)))
	}

	ruleset := rulesetList[0]
	ruleset.Name = name
	return ruleset
}
