/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdserver

import (
	"fmt"
	"net/http"
	"path"

	"github.com/coreos/etcd/pkg/types"
)

// ServerConfig holds the configuration of etcd as taken from the command line or discovery.
type ServerConfig struct {
	Name         string
	DiscoveryURL string
	ClientURLs   types.URLs
	DataDir      string
	SnapCount    uint64
	Cluster      *Cluster
	ClusterState ClusterState
	Transport    *http.Transport
}

// Verify sanity-checks the config struct and returns an error for things that
// should never happen.
func (c *ServerConfig) Verify() error {
	// Make sure the cluster at least contains the local server.
	m := c.Cluster.FindName(c.Name)
	if m == nil {
		return fmt.Errorf("could not find name %v in cluster", c.Name)
	}

	// No identical IPs in the cluster peer list
	urlMap := make(map[string]bool)
	for _, m := range *c.Cluster {
		for _, url := range m.PeerURLs {
			if urlMap[url] {
				return fmt.Errorf("duplicate url %v in server config", url)
			}
			urlMap[url] = true
		}
	}
	return nil
}

func (c *ServerConfig) WALDir() string { return path.Join(c.DataDir, "wal") }

func (c *ServerConfig) SnapDir() string { return path.Join(c.DataDir, "snap") }

func (c *ServerConfig) ID() uint64 { return c.Cluster.FindName(c.Name).ID }

func (c *ServerConfig) ShouldDiscover() bool {
	return c.DiscoveryURL != ""
}

// IsBootstrap returns true if a bootstrap method is provided.
func (c *ServerConfig) IsBootstrap() bool {
	return c.DiscoveryURL != "" || c.ClusterState == ClusterStateValueNew
}
