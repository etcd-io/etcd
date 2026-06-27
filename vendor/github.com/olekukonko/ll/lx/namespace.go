package lx

import (
	"strings"
	"sync"
)

// namespaceRule stores the cached result of Enabled.
type namespaceRule struct {
	isEnabledByRule  bool
	isDisabledByRule bool
}

// Namespace manages thread-safe namespace enable/disable states with caching.
// The store holds explicit user-defined rules (path -> bool).
// The cache holds computed effective states for paths (path -> namespaceRule)
// based on hierarchical rules to optimize lookups.
type Namespace struct {
	store sync.Map // path (string) -> rule (bool: true=enable, false=disable)
	cache sync.Map // path (string) -> namespaceRule
}

// Set defines an explicit enable/disable rule for a namespace path.
// It clears the cache to ensure subsequent lookups reflect the change.
func (ns *Namespace) Set(path string, enabled bool) {
	ns.store.Store(path, enabled)
	ns.clearCache()
}

// Load retrieves an explicit rule from the store for a path.
// Returns the rule (true=enable, false=disable) and whether it exists.
// Does not consider hierarchy or caching.
func (ns *Namespace) Load(path string) (rule interface{}, found bool) {
	return ns.store.Load(path)
}

// Store directly sets a rule in the store, bypassing cache invalidation.
// Intended for internal use or sync.Map parity; prefer Set for standard use.
func (ns *Namespace) Store(path string, rule bool) {
	ns.store.Store(path, rule)
}

// clearCache clears the cache of Enabled results.
// Called by Set to ensure consistency after rule changes.
func (ns *Namespace) clearCache() {
	ns.cache.Range(func(key, _ interface{}) bool {
		ns.cache.Delete(key)
		return true
	})
}

// Enabled checks if a path is enabled by namespace rules, considering the most
// specific rule (path or closest prefix) in the store. Results are cached.
// Args:
//   - path: Absolute namespace path to check.
//   - separator: Character delimiting path segments (e.g., "/", ".").
//
// Returns:
//   - isEnabledByRule: True if an explicit rule enables the path.
//   - isDisabledByRule: True if an explicit rule disables the path.
//
// If both are false, no explicit rule applies to the path or its prefixes.
func (ns *Namespace) Enabled(path string, separator string) (isEnabledByRule bool, isDisabledByRule bool) {
	if path == "" { // Root path has no explicit rule
		return false, false
	}

	// Check cache
	if cachedValue, found := ns.cache.Load(path); found {
		if state, ok := cachedValue.(namespaceRule); ok {
			return state.isEnabledByRule, state.isDisabledByRule
		}
		ns.cache.Delete(path) // Remove invalid cache entry
	}

	// Compute: Most specific rule wins
	parts := strings.Split(path, separator)
	computedIsEnabled := false
	computedIsDisabled := false

	for i := len(parts); i >= 1; i-- {
		currentPrefix := strings.Join(parts[:i], separator)
		if val, ok := ns.store.Load(currentPrefix); ok {
			if rule := val.(bool); rule {
				computedIsEnabled = true
				computedIsDisabled = false
			} else {
				computedIsEnabled = false
				computedIsDisabled = true
			}
			break
		}
	}

	// Cache result, including (false, false) for no rule
	ns.cache.Store(path, namespaceRule{
		isEnabledByRule:  computedIsEnabled,
		isDisabledByRule: computedIsDisabled,
	})

	return computedIsEnabled, computedIsDisabled
}
