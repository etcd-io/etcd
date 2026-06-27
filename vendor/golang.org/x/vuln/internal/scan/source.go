// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"context"
	"fmt"

	"golang.org/x/tools/go/packages"
	"golang.org/x/vuln/internal/client"
	"golang.org/x/vuln/internal/derrors"
	"golang.org/x/vuln/internal/govulncheck"
	"golang.org/x/vuln/internal/vulncheck"
)

// runSource reports vulnerabilities that affect the analyzed packages.
//
// Vulnerabilities can be called (affecting the package, because a vulnerable
// symbol is actually exercised) or just imported by the package
// (likely having a non-affecting outcome).
func runSource(ctx context.Context, handler govulncheck.Handler, cfg *config, client *client.Client, dir string) (err error) {
	defer derrors.Wrap(&err, "govulncheck")

	if cfg.ScanLevel.WantPackages() && len(cfg.patterns) == 0 {
		return errNoPatterns
	}
	if !gomodExists(dir) {
		return errNoGoMod
	}
	graph := vulncheck.NewPackageGraph(cfg.GoVersion)
	pkgConfig := &packages.Config{
		Dir:   dir,
		Tests: cfg.test,
		Env:   cfg.env,
	}
	if err := graph.LoadPackagesAndMods(pkgConfig, cfg.tags, cfg.patterns, cfg.ScanLevel == govulncheck.ScanLevelSymbol); err != nil {
		if isGoVersionMismatchError(err) {
			return fmt.Errorf("%v\n\n%v", errGoVersionMismatch, err)
		}
		return fmt.Errorf("loading packages: %w", err)
	}

	if cfg.ScanLevel.WantPackages() && len(graph.TopPkgs()) == 0 {
		return errNoPackagesMatched
	}
	return vulncheck.Source(ctx, handler, &cfg.Config, client, graph)
}
