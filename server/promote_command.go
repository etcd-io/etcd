package server

import (
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&PromoteCommand{})
}

// PromoteCommand represents a Raft command for converting a proxy to a peer.
type PromoteCommand struct {
	Name string `json:"name"`
}

// CommandName returns the name of the command.
func (c *PromoteCommand) CommandName() string {
	return "etcd:promote"
}

// Apply promotes a named proxy to a peer.
func (c *PromoteCommand) Apply(context raft.Context) (interface{}, error) {
	ps, _ := context.Server().Context().(*PeerServer)
	config := ps.ClusterConfig()

	// If cluster size is larger than max cluster size then return an error.
	if ps.registry.PeerCount() >= config.ActiveSize {
		return etcdErr.NewError(etcdErr.EcodePromoteError, "", 0)
	}

	// If proxy doesn't exist then return an error.
	if !ps.registry.ProxyExists(c.Name) {
		return etcdErr.NewError(etcdErr.EcodePromoteError, "", 0)
	}

	// Retrieve proxy settings.
	proxyClientURL := ps.registry.ProxyClientURL()
	proxyPeerURL := ps.registry.ProxyPeerURL()

	// Remove from registry as a proxy.
	if err := ps.registry.UnregisterProxy(c.Name); err != nil {
		log.Info("Cannot remove proxy: ", c.Name)
		return nil, err
	}

	// Add to shared peer registry.
	ps.registry.RegisterPeer(c.Name, c.RaftURL, c.EtcdURL)

	// Add peer in raft
	err := context.Server().AddPeer(c.Name, "")

	// Add peer stats
	if c.Name != ps.RaftServer().Name() {
		ps.followersStats.Followers[c.Name] = &raftFollowerStats{}
		ps.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	return nil, err
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}
