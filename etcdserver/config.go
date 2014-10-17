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

// Verify sanity-checks the config struct and returns an error for things that
// should never happen.
func (c *ServerConfig) Verify() error {
	// Make sure the cluster at least contains the local server.
	m := c.Cluster.FindName(c.Name)
	if m == nil {
		return fmt.Errorf("could not find name %v in cluster", c.Name)
	}
	if m.ID == raft.None {
		return fmt.Errorf("cannot use None(%x) as member id", raft.None)
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
