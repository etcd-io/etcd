package linter

import (
	"go/ast"

	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/internal/cache"
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis/load"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

type Context struct {
	// Packages are deduplicated (test and normal packages) packages
	Packages []*packages.Package

	// OriginalPackages aren't deduplicated: they contain both normal and test
	// version for each of packages
	OriginalPackages []*packages.Package

	Cfg *config.Config
	Log logutils.Log

	PkgCache  *cache.Cache
	LoadGuard *load.Guard
}

func (c *Context) Settings() *config.LintersSettings {
	return &c.Cfg.Linters.Settings
}

func (c *Context) ClearTypesInPackages() {
	for _, p := range c.Packages {
		clearTypes(p)
	}
	for _, p := range c.OriginalPackages {
		clearTypes(p)
	}
}

func clearTypes(p *packages.Package) {
	p.Types = nil
	p.TypesInfo = nil
	p.Syntax = []*ast.File{}
}
