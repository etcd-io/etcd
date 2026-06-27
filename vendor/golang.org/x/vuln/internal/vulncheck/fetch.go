// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"context"
	"fmt"

	"golang.org/x/tools/go/packages"
	"golang.org/x/vuln/internal/client"
)

// FetchVulnerabilities fetches vulnerabilities that affect the supplied modules.
func FetchVulnerabilities(ctx context.Context, c *client.Client, modules []*packages.Module) ([]*ModVulns, error) {
	mreqs := make([]*client.ModuleRequest, len(modules))
	for i, mod := range modules {
		modPath := mod.Path
		if mod.Replace != nil {
			modPath = mod.Replace.Path
		}
		mreqs[i] = &client.ModuleRequest{
			Path: modPath,
		}
	}
	resps, err := c.ByModules(ctx, mreqs)
	if err != nil {
		return nil, fmt.Errorf("fetching vulnerabilities: %v", err)
	}
	var mv []*ModVulns
	for i, resp := range resps {
		if len(resp.Entries) == 0 {
			continue
		}
		mv = append(mv, &ModVulns{
			Module: modules[i],
			Vulns:  resp.Entries,
		})
	}
	return mv, nil
}
