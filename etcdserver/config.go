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
	"github.com/coreos/etcd/raft"
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

// VerifyBootstrapConfig sanity-checks the initial config and returns an error
// for things that should never happen.
func (c *ServerConfig) VerifyBootstrapConfig() error {
	m := c.Cluster.MemberFromName(c.Name)
	// Make sure the cluster at least contains the local server.
	if m == nil {
		return fmt.Errorf("couldn't find local name %s in the initial cluster configuration", c.Name)
	}
	if m.ID == raft.None {
		return fmt.Errorf("could not use %x as member id", raft.None)
	}

	if c.DiscoveryURL == "" && c.ClusterState != ClusterStateValueNew {
		return fmt.Errorf("initial cluster state unset and no wal or discovery URL found")
	}

	// No identical IPs in the cluster peer list
	urlMap := make(map[string]bool)
	for _, m := range c.Cluster.Members() {
		for _, url := range m.PeerURLs {
			if urlMap[url] {
				return fmt.Errorf("duplicate url %v in cluster config", url)
			}
			urlMap[url] = true
		}
	}
	return nil
}

func (c *ServerConfig) WALDir() string { return path.Join(c.DataDir, "wal") }

func (c *ServerConfig) SnapDir() string { return path.Join(c.DataDir, "snap") }

func (c *ServerConfig) ShouldDiscover() bool {
	return c.DiscoveryURL != ""
}
