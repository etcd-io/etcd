// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"fmt"
	"go/token"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
	"golang.org/x/vuln/internal"
	"golang.org/x/vuln/internal/osv"
	"golang.org/x/vuln/internal/semver"
)

const (
	fetchingVulnsMessage    = "Fetching vulnerabilities from the database..."
	checkingSrcVulnsMessage = "Checking the code against the vulnerabilities..."
	checkingBinVulnsMessage = "Checking the binary against the vulnerabilities..."
)

// Result contains information on detected vulnerabilities.
// For call graph analysis, it provides information on reachability
// of vulnerable symbols through entry points of the program.
type Result struct {
	// EntryFunctions are a subset of Functions representing vulncheck entry points.
	EntryFunctions []*FuncNode

	// Vulns contains information on detected vulnerabilities.
	Vulns []*Vuln
}

// Vuln provides information on a detected vulnerability. For call
// graph mode, Vuln will also contain the information on how the
// vulnerability is reachable in the user call graph.
type Vuln struct {
	// OSV contains information on the detected vulnerability in the shared
	// vulnerability format.
	//
	// OSV, Symbol, and Package identify a vulnerability.
	//
	// Note that *osv.Entry may describe multiple symbols from multiple
	// packages.
	OSV *osv.Entry

	// Symbol is the name of the detected vulnerable function or method.
	Symbol string

	// CallSink is the FuncNode corresponding to Symbol.
	//
	// When analyzing binaries, Symbol is not reachable, or cfg.ScanLevel
	// is symbol, CallSink will be unavailable and set to nil.
	CallSink *FuncNode

	// Package of Symbol.
	//
	// When the package of symbol is not imported, Package will be
	// unavailable and set to nil.
	Package *packages.Package
}

// A FuncNode describes a function in the call graph.
type FuncNode struct {
	// Name is the name of the function.
	Name string

	// RecvType is the receiver object type of this function, if any.
	RecvType string

	// Package is the package the function is part of.
	Package *packages.Package

	// Position describes the position of the function in the file.
	Pos *token.Position

	// CallSites is a set of call sites where this function is called.
	CallSites []*CallSite
}

func (fn *FuncNode) String() string {
	if fn.RecvType == "" {
		return fmt.Sprintf("%s.%s", fn.Package.PkgPath, fn.Name)
	}
	return fmt.Sprintf("%s.%s", fn.RecvType, fn.Name)
}

// Receiver returns the FuncNode's receiver, with package path removed.
// Pointers are preserved if present.
func (fn *FuncNode) Receiver() string {
	return strings.Replace(fn.RecvType, fmt.Sprintf("%s.", fn.Package.PkgPath), "", 1)
}

// A CallSite describes a function call.
type CallSite struct {
	// Parent is the enclosing function where the call is made.
	Parent *FuncNode

	// Name stands for the name of the function (variable) being called.
	Name string

	// RecvType is the full path of the receiver object type, if any.
	RecvType string

	// Position describes the position of the function in the file.
	Pos *token.Position

	// Resolved indicates if the called function can be statically resolved.
	Resolved bool
}

// affectingVulns is an internal structure for querying
// vulnerabilities that apply to the current program
// and platform under consideration.
type affectingVulns []*ModVulns

// ModVulns groups vulnerabilities per module.
type ModVulns struct {
	Module *packages.Module
	Vulns  []*osv.Entry
}

func affectingVulnerabilities(vulns []*ModVulns, os, arch string) affectingVulns {
	now := time.Now()
	var filtered affectingVulns
	for _, mod := range vulns {
		module := mod.Module
		modVersion := module.Version
		if module.Replace != nil {
			modVersion = module.Replace.Version
		}
		// TODO(https://golang.org/issues/49264): if modVersion == "", try vcs?
		var filteredVulns []*osv.Entry
		for _, v := range mod.Vulns {
			// Ignore vulnerabilities that have been withdrawn
			if v.Withdrawn != nil && v.Withdrawn.Before(now) {
				continue
			}

			var filteredAffected []osv.Affected
			for _, a := range v.Affected {
				// Vulnerabilities from some databases might contain
				// information on related but different modules that
				// were, say, reported in the same CVE. We filter such
				// information out as it might lead to incorrect results:
				// Computing a latest fix could consider versions of these
				// different packages.
				if a.Module.Path != module.Path {
					continue
				}
				if !affected(modVersion, a) {
					continue
				}

				var filteredImports []osv.Package
				for _, p := range a.EcosystemSpecific.Packages {
					if matchesPlatform(os, arch, p) {
						filteredImports = append(filteredImports, p)
					}
				}
				// If we pruned all existing Packages, then the affected is
				// empty and we can filter it out. Note that Packages can
				// be empty for vulnerabilities that have no package or
				// symbol information available.
				if len(a.EcosystemSpecific.Packages) != 0 && len(filteredImports) == 0 {
					continue
				}
				a.EcosystemSpecific.Packages = filteredImports
				filteredAffected = append(filteredAffected, a)
			}
			if len(filteredAffected) == 0 {
				continue
			}
			// save the non-empty vulnerability with only
			// affected symbols.
			newV := *v
			newV.Affected = filteredAffected
			filteredVulns = append(filteredVulns, &newV)
		}

		filtered = append(filtered, &ModVulns{
			Module: module,
			Vulns:  filteredVulns,
		})
	}
	return filtered
}

// affected checks if modVersion is affected by a:
//   - it is included in one of the affected version ranges
//   - and module version is not "" and "(devel)"
func affected(modVersion string, a osv.Affected) bool {
	const devel = "(devel)"
	if modVersion == "" || modVersion == devel {
		// Module version of "" means the module version is not available
		// and devel means it is in development stage. Either way, we don't
		// know the exact version so we don't want to spam users with
		// potential false alarms.
		return false
	}
	return semver.Affects(a.Ranges, modVersion)
}

func matchesPlatform(os, arch string, e osv.Package) bool {
	return matchesPlatformComponent(os, e.GOOS) &&
		matchesPlatformComponent(arch, e.GOARCH)
}

// matchesPlatformComponent reports whether a GOOS (or GOARCH)
// matches a list of GOOS (or GOARCH) values from an osv.EcosystemSpecificImport.
func matchesPlatformComponent(s string, ps []string) bool {
	// An empty input or an empty GOOS or GOARCH list means "matches everything."
	if s == "" || len(ps) == 0 {
		return true
	}
	for _, p := range ps {
		if s == p {
			return true
		}
	}
	return false
}

// moduleVulns return vulnerabilities for module. If module is unknown,
// it figures the module from package importPath. It returns the module
// whose path is the longest prefix of importPath.
func (aff affectingVulns) moduleVulns(module, importPath string) *ModVulns {
	moduleKnown := module != "" && module != internal.UnknownModulePath

	isStd := IsStdPackage(importPath)
	var mostSpecificMod *ModVulns // for the case where !moduleKnown
	for _, mod := range aff {
		md := mod
		if isStd && mod.Module.Path == internal.GoStdModulePath {
			// Standard library packages do not have an associated module,
			// so we relate them to the artificial stdlib module.
			return md
		}

		if moduleKnown {
			if mod.Module.Path == module {
				// If we know exactly which module we need,
				// return its vulnerabilities.
				return md
			}
		} else if strings.HasPrefix(importPath, md.Module.Path) {
			// If module is unknown, we try to figure it out from importPath.
			// We take the module whose path has the longest match to importPath.
			// TODO: do matching based on path components.
			if mostSpecificMod == nil || len(mostSpecificMod.Module.Path) < len(md.Module.Path) {
				mostSpecificMod = md
			}
		}
	}
	return mostSpecificMod
}

// ForPackage returns the vulnerabilities for the importPath belonging to
// module.
//
// If module is unknown, ForPackage will resolve it as the most specific
// prefix of importPath.
func (aff affectingVulns) ForPackage(module, importPath string) []*osv.Entry {
	mod := aff.moduleVulns(module, importPath)
	if mod == nil {
		return nil
	}

	if mod.Module.Replace != nil {
		// standard libraries do not have a module nor replace module
		importPath = fmt.Sprintf("%s%s", mod.Module.Replace.Path, strings.TrimPrefix(importPath, mod.Module.Path))
	}
	vulns := mod.Vulns
	packageVulns := []*osv.Entry{}
Vuln:
	for _, v := range vulns {
		for _, a := range v.Affected {
			if len(a.EcosystemSpecific.Packages) == 0 {
				// no packages means all packages are vulnerable
				packageVulns = append(packageVulns, v)
				continue Vuln
			}

			for _, p := range a.EcosystemSpecific.Packages {
				if p.Path == importPath {
					packageVulns = append(packageVulns, v)
					continue Vuln
				}
			}
		}
	}
	return packageVulns
}

// ForSymbol returns vulnerabilities for symbol in aff.ForPackage(module, importPath).
func (aff affectingVulns) ForSymbol(module, importPath, symbol string) []*osv.Entry {
	vulns := aff.ForPackage(module, importPath)
	if vulns == nil {
		return nil
	}

	symbolVulns := []*osv.Entry{}
vulnLoop:
	for _, v := range vulns {
		for _, a := range v.Affected {
			if len(a.EcosystemSpecific.Packages) == 0 {
				// no packages means all symbols of all packages are vulnerable
				symbolVulns = append(symbolVulns, v)
				continue vulnLoop
			}

			for _, p := range a.EcosystemSpecific.Packages {
				if p.Path != importPath {
					continue
				}
				if len(p.Symbols) > 0 && !contains(p.Symbols, symbol) {
					continue
				}
				symbolVulns = append(symbolVulns, v)
				continue vulnLoop
			}
		}
	}
	return symbolVulns
}

func contains(symbols []string, target string) bool {
	for _, s := range symbols {
		if s == target {
			return true
		}
	}
	return false
}
