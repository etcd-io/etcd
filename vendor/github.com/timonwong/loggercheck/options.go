package loggercheck

import (
	"github.com/timonwong/loggercheck/internal/sets"
)

type Option func(*loggercheck)

func WithDisable(disable []string) Option {
	return func(l *loggercheck) {
		l.disable = sets.NewString(disable...)
	}
}

func WithRules(customRules []string) Option {
	return func(l *loggercheck) {
		l.rules = customRules
	}
}

func WithRequireStringKey(requireStringKey bool) Option {
	return func(l *loggercheck) {
		l.requireStringKey = requireStringKey
	}
}

func WithNoPrintfLike(noPrintfLike bool) Option {
	return func(l *loggercheck) {
		l.noPrintfLike = noPrintfLike
	}
}
