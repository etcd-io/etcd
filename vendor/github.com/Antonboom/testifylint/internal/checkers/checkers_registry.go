package checkers

import (
	"sort"
)

// registry stores checkers meta information in checkers' priority order.
var registry = checkersRegistry{
	// Regular checkers.
	{factory: asCheckerFactory(NewFloatCompare), enabledByDefault: true},
	{factory: asCheckerFactory(NewBoolCompare), enabledByDefault: true},
	{factory: asCheckerFactory(NewEmpty), enabledByDefault: true},
	{factory: asCheckerFactory(NewNegativePositive), enabledByDefault: true},
	{factory: asCheckerFactory(NewCompares), enabledByDefault: true},
	{factory: asCheckerFactory(NewContains), enabledByDefault: true},
	{factory: asCheckerFactory(NewErrorNil), enabledByDefault: true},
	{factory: asCheckerFactory(NewNilCompare), enabledByDefault: true},
	{factory: asCheckerFactory(NewErrorIsAs), enabledByDefault: true},
	{factory: asCheckerFactory(NewEncodedCompare), enabledByDefault: true},
	{factory: asCheckerFactory(NewExpectedActual), enabledByDefault: true},
	{factory: asCheckerFactory(NewLen), enabledByDefault: true},
	{factory: asCheckerFactory(NewEqualValues), enabledByDefault: true},
	{factory: asCheckerFactory(NewRegexp), enabledByDefault: true},
	{factory: asCheckerFactory(NewSuiteExtraAssertCall), enabledByDefault: true},
	{factory: asCheckerFactory(NewSuiteDontUsePkg), enabledByDefault: true},
	{factory: asCheckerFactory(NewUselessAssert), enabledByDefault: true},
	{factory: asCheckerFactory(NewFormatter), enabledByDefault: true},
	// Advanced checkers.
	{factory: asCheckerFactory(NewBlankImport), enabledByDefault: true},
	{factory: asCheckerFactory(NewGoRequire), enabledByDefault: true},
	{factory: asCheckerFactory(NewRequireError), enabledByDefault: true},
	{factory: asCheckerFactory(NewSuiteBrokenParallel), enabledByDefault: true},
	{factory: asCheckerFactory(NewSuiteMethodSignature), enabledByDefault: true},
	{factory: asCheckerFactory(NewSuiteSubtestRun), enabledByDefault: true},
	{factory: asCheckerFactory(NewSuiteTHelper), enabledByDefault: false},
}

type checkersRegistry []checkerMeta

type checkerMeta struct {
	factory          checkerFactory
	enabledByDefault bool
}

type checkerFactory func() Checker

func asCheckerFactory[T Checker](fn func() T) checkerFactory {
	return func() Checker {
		return fn()
	}
}

func (r checkersRegistry) get(name string) (m checkerMeta, priority int, found bool) {
	for i, meta := range r {
		if meta.factory().Name() == name {
			return meta, i, true
		}
	}
	return checkerMeta{}, 0, false
}

// All returns all checkers names sorted by checker's priority.
func All() []string {
	result := make([]string, 0, len(registry))
	for _, meta := range registry {
		result = append(result, meta.factory().Name())
	}
	return result
}

// EnabledByDefault returns checkers enabled by default sorted by checker's priority.
func EnabledByDefault() []string {
	result := make([]string, 0, len(registry))
	for _, meta := range registry {
		if meta.enabledByDefault {
			result = append(result, meta.factory().Name())
		}
	}
	return result
}

// Get returns new checker instance by checker's name.
func Get(name string) (Checker, bool) {
	meta, _, ok := registry.get(name)
	if ok {
		return meta.factory(), true
	}
	return nil, false
}

// IsKnown checks if there is a checker with that name.
func IsKnown(name string) bool {
	_, _, ok := registry.get(name)
	return ok
}

// IsEnabledByDefault returns true if a checker is enabled by default.
// Returns false if there is no such checker in the registry.
// For pre-validation use Get or IsKnown.
func IsEnabledByDefault(name string) bool {
	meta, _, ok := registry.get(name)
	return ok && meta.enabledByDefault
}

// SortByPriority mutates the input checkers names by sorting them in checker priority order.
// Ignores unknown checkers. For pre-validation use Get or IsKnown.
func SortByPriority(checkers []string) {
	sort.Slice(checkers, func(i, j int) bool {
		lhs, rhs := checkers[i], checkers[j]
		_, lhsPriority, _ := registry.get(lhs)
		_, rhsPriority, _ := registry.get(rhs)
		return lhsPriority < rhsPriority
	})
}
