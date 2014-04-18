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

func (c *JoinCommandV1) updatePeerURL(ps *PeerServer) error {
	log.Debugf("Update peer URL of %v to %v", c.Name, c.RaftURL)
	if err := ps.registry.UpdatePeerURL(c.Name, c.RaftURL); err != nil {
		log.Debugf("Error while updating in registry: %s (%v)", c.Name, err)
		return err
	}
	// Flush commit index, so raft will replay to here when restarted
	ps.raftServer.FlushCommitIndex()
	return nil
}

// Join a server to the cluster
func (c *JoinCommandV1) Apply(context raft.Context) (interface{}, error) {
	ps, _ := context.Server().Context().(*PeerServer)

	b := make([]byte, 8)
	binary.PutUvarint(b, context.CommitIndex())

	// Make sure we're not getting a cached value from the registry.
	ps.registry.Invalidate(c.Name)

	// Check if the join command is from a previous peer, who lost all its previous log.
	if peerURL, ok := ps.registry.PeerURL(c.Name); ok {
		// If previous node restarts with different peer URL,
		// update its information.
		if peerURL != c.RaftURL {
			log.Infof("Rejoin with %v instead of %v from %v", c.RaftURL, peerURL, c.Name)
			if err := c.updatePeerURL(ps); err != nil {
				return []byte{0}, err
			}
		}
		return b, nil
	}

	// Check if the join command adds an instance that collides with existing one on peer URL.
	peerURLs := ps.registry.PeerURLs(ps.raftServer.Leader(), c.Name)
	for _, peerURL := range peerURLs {
		if peerURL == c.RaftURL {
			log.Warnf("%v tries to join the cluster with existing URL %v", c.Name, c.EtcdURL)
			return []byte{0}, etcdErr.NewError(etcdErr.EcodeExistingPeerAddr, c.EtcdURL, context.CommitIndex())
		}
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

func (c *JoinCommandV2) updatePeerURL(ps *PeerServer) error {
	log.Debugf("Update peer URL of %v to %v", c.Name, c.PeerURL)
	if err := ps.registry.UpdatePeerURL(c.Name, c.PeerURL); err != nil {
		log.Debugf("Error while updating in registry: %s (%v)", c.Name, err)
		return err
	}
	// Flush commit index, so raft will replay to here when restart
	ps.raftServer.FlushCommitIndex()
	return nil
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
	if peerURL, ok := ps.registry.PeerURL(c.Name); ok {
		// If previous node restarts with different peer URL,
		// update its information.
		if peerURL != c.PeerURL {
			log.Infof("Rejoin with %v instead of %v from %v", c.PeerURL, peerURL, c.Name)
			if err := c.updatePeerURL(ps); err != nil {
				return []byte{0}, err
			}
		}
		return json.Marshal(msg)
	}

	// Check if the join command adds an instance that collides with existing one on peer URL.
	peerURLs := ps.registry.PeerURLs(ps.raftServer.Leader(), c.Name)
	for _, peerURL := range peerURLs {
		if peerURL == c.PeerURL {
			log.Warnf("%v tries to join the cluster with existing URL %v", c.Name, c.PeerURL)
			return []byte{0}, etcdErr.NewError(etcdErr.EcodeExistingPeerAddr, c.PeerURL, context.CommitIndex())
		}
	}

	// Check peer number in the cluster.
	if ps.registry.PeerCount() >= ps.ClusterConfig().ActiveSize {
		log.Debug("Join as standby ", c.Name)
		ps.registry.RegisterStandby(c.Name, c.PeerURL, c.ClientURL)
		msg.Mode = StandbyMode
		return json.Marshal(msg)
	}

	// Remove it as a standby if it is one.
	if ps.registry.StandbyExists(c.Name) {
		ps.registry.UnregisterStandby(c.Name)
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
