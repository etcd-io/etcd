package gomodguard

import "strings"

// Allowed is a list of modules and module
// domains that are allowed to be used.
type Allowed struct {
	Modules []string `yaml:"modules"`
	Domains []string `yaml:"domains"`
}

// IsAllowedModule returns true if the given module
// name is in the allowed modules list.
func (a *Allowed) IsAllowedModule(moduleName string) bool {
	allowedModules := a.Modules

	for i := range allowedModules {
		if strings.TrimSpace(moduleName) == strings.TrimSpace(allowedModules[i]) {
			return true
		}
	}

	return false
}

// IsAllowedModuleDomain returns true if the given modules domain is
// in the allowed module domains list.
func (a *Allowed) IsAllowedModuleDomain(moduleName string) bool {
	allowedDomains := a.Domains

	for i := range allowedDomains {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(moduleName)),
			strings.TrimSpace(strings.ToLower(allowedDomains[i]))) {
			return true
		}
	}

	return false
}
