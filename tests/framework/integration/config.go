// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import "go.etcd.io/etcd/tests/v3/framework/config"

type ClusterContext struct {
	UseUnix bool
}

func WithHTTP2Debug() config.ClusterOption {
	return func(c *config.ClusterConfig) {}
}

func WithUnixClient() config.ClusterOption {
	return func(c *config.ClusterConfig) {
		ctx := ensureIntegrationClusterContext(c)
		ctx.UseUnix = true
		c.ClusterContext = ctx
	}
}

func WithTCPClient() config.ClusterOption {
	return func(c *config.ClusterConfig) {
		ctx := ensureIntegrationClusterContext(c)
		ctx.UseUnix = false
		c.ClusterContext = ctx
	}
}

func ensureIntegrationClusterContext(c *config.ClusterConfig) *ClusterContext {
	ctx, _ := c.ClusterContext.(*ClusterContext)
	if ctx == nil {
		ctx = &ClusterContext{}
	}
	return ctx
}
