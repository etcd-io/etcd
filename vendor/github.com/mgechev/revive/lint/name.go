package lint

import "github.com/mgechev/revive/internal/rule"

// Name returns a different name if it should be different.
//
// Deprecated: Do not use this function, it will be removed in the next major release.
func Name(name string, allowlist, blocklist []string) string {
	return rule.Name(name, allowlist, blocklist, false)
}
