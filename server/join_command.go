package server

import (
	"encoding/binary"
	"encoding/json"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

func init() {
	raft.RegisterCommand(&JoinCommandV1{})
	raft.RegisterCommand(&JoinCommandV2{})
}

// JoinCommandV1 represents a request to join the cluster.
// The command returns the join_index (Uvarint).
type JoinCommandV1 struct {
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
	Name       string `json:"name"`
	RaftURL    string `json:"raftURL"`
	EtcdURL    string `json:"etcdURL"`
}

// The name of the join command in the log
func (c *JoinCommandV1) CommandName() string {
	return "etcd:join"
}

// Join a server to the cluster
func (c *JoinCommandV1) Apply(context raft.Context) (interface{}, error) {
	ps, _ := context.Server().Context().(*PeerServer)

	b := make([]byte, 8)
	binary.PutUvarint(b, context.CommitIndex())

	// Make sure we're not getting a cached value from the registry.
	ps.registry.Invalidate(c.Name)

	// Check if the join command is from a previous peer, who lost all its previous log.
	if _, ok := ps.registry.ClientURL(c.Name); ok {
		return b, nil
	}

	// Check peer number in the cluster
	if ps.registry.PeerCount() >= ps.ClusterConfig().ActiveSize {
		log.Debug("Reject join request from ", c.Name)
		return []byte{0}, etcdErr.NewError(etcdErr.EcodeNoMorePeer, "", context.CommitIndex())
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

	return b, err
}

func (c *JoinCommandV1) NodeName() string {
	return c.Name
}

// JoinCommandV2 represents a request to join the cluster.
type JoinCommandV2 struct {
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
	Name       string `json:"name"`
	PeerURL    string `json:"peerURL"`
	ClientURL  string `json:"clientURL"`
}

// CommandName returns the name of the command in the Raft log.
func (c *JoinCommandV2) CommandName() string {
	return "etcd:v2:join"
}

// Apply attempts to join a machine to the cluster.
func (c *JoinCommandV2) Apply(context raft.Context) (interface{}, error) {
	ps, _ := context.Server().Context().(*PeerServer)
	var msg = joinMessageV2{
		Mode:        PeerMode,
		CommitIndex: context.CommitIndex(),
	}

	// Make sure we're not getting a cached value from the registry.
	ps.registry.Invalidate(c.Name)

	// Check if the join command is from a previous peer, who lost all its previous log.
	if _, ok := ps.registry.ClientURL(c.Name); ok {
		return json.Marshal(msg)
	}

	// Check peer number in the cluster.
	if ps.registry.PeerCount() >= ps.ClusterConfig().ActiveSize {
		log.Debug("Join as proxy ", c.Name)
		ps.registry.RegisterProxy(c.Name, c.PeerURL, c.ClientURL)
		msg.Mode = ProxyMode
		return json.Marshal(msg)
	}

	// Remove it as a proxy if it is one.
	if ps.registry.ProxyExists(c.Name) {
		ps.registry.UnregisterProxy(c.Name)
	}

	// Add to shared peer registry.
	ps.registry.RegisterPeer(c.Name, c.PeerURL, c.ClientURL)

	// Add peer in raft
	if err := context.Server().AddPeer(c.Name, ""); err != nil {
		b, _ := json.Marshal(msg)
		return b, err
	}

	// Add peer stats
	if c.Name != ps.RaftServer().Name() {
		ps.followersStats.Followers[c.Name] = &raftFollowerStats{}
		ps.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	return json.Marshal(msg)
}

func (c *JoinCommandV2) NodeName() string {
	return c.Name
}

type joinMessageV2 struct {
	CommitIndex uint64 `json:"commitIndex"`
	Mode        Mode   `json:"mode"`
}
