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
	"log"
	"net/http"
	"path"
	"reflect"
	"sort"

	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
)

// ServerConfig holds the configuration of etcd as taken from the command line or discovery.
type ServerConfig struct {
	Name            string
	DiscoveryURL    string
	DiscoveryProxy  string
	ClientURLs      types.URLs
	PeerURLs        types.URLs
	DataDir         string
	SnapCount       uint64
	MaxSnapFiles    uint
	MaxWALFiles     uint
	Cluster         *Cluster
	NewCluster      bool
	ForceNewCluster bool
	Transport       *http.Transport
}

// VerifyBootstrapConfig sanity-checks the initial config and returns an error
// for things that should never happen.
func (c *ServerConfig) VerifyBootstrapConfig() error {
	m := c.Cluster.MemberByName(c.Name)
	// Make sure the cluster at least contains the local server.
	if m == nil {
		return fmt.Errorf("couldn't find local name %q in the initial cluster configuration", c.Name)
	}
	if uint64(m.ID) == raft.None {
		return fmt.Errorf("cannot use %x as member id", raft.None)
	}

	if c.DiscoveryURL == "" && !c.NewCluster {
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

	// Advertised peer URLs must match those in the cluster peer list
	apurls := c.PeerURLs.StringSlice()
	sort.Strings(apurls)
	if !reflect.DeepEqual(apurls, m.PeerURLs) {
		return fmt.Errorf("%s has different advertised URLs in the cluster and advertised peer URLs list", c.Name)
	}
	return nil
}

func (c *ServerConfig) WALDir() string { return path.Join(c.DataDir, "wal") }

func (c *ServerConfig) SnapDir() string { return path.Join(c.DataDir, "snap") }

func (c *ServerConfig) ShouldDiscover() bool { return c.DiscoveryURL != "" }

func (c *ServerConfig) PrintWithInitial() { c.print(true) }

func (c *ServerConfig) Print() { c.print(false) }

func (c *ServerConfig) print(initial bool) {
	log.Printf("etcdserver: name = %s", c.Name)
	if c.ForceNewCluster {
		log.Println("etcdserver: force new cluster")
	}
	log.Printf("etcdserver: data dir = %s", c.DataDir)
	log.Printf("etcdserver: snapshot count = %d", c.SnapCount)
	if len(c.DiscoveryURL) != 0 {
		log.Printf("etcdserver: discovery URL= %s", c.DiscoveryURL)
		if len(c.DiscoveryProxy) != 0 {
			log.Printf("etcdserver: discovery proxy = %s", c.DiscoveryProxy)
		}
	}
	log.Printf("etcdserver: advertise client URLs = %s", c.ClientURLs)
	if initial {
		log.Printf("etcdserver: initial advertise peer URLs = %s", c.PeerURLs)
		log.Printf("etcdserver: initial cluster = %s", c.Cluster)
	}
}
