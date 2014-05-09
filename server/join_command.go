package server

import (
	"encoding/binary"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

func init() {
	raft.RegisterCommand(&JoinCommand{})
}

// JoinCommand represents a request to join the cluster.
// The command returns the join_index (Uvarint).
type JoinCommand struct {
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
	Name       string `json:"name"`
	RaftURL    string `json:"raftURL"`
	EtcdURL    string `json:"etcdURL"`
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return "etcd:join"
}

// Apply attempts to join a machine to the cluster.
func (c *JoinCommand) Apply(context raft.Context) (interface{}, error) {
	index, err := applyJoin(c, context)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 8)
	binary.PutUvarint(b, index)
	return b, nil
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}

// applyJoin attempts to join a machine to the cluster.
func applyJoin(c *JoinCommand, context raft.Context) (uint64, error) {
	ps, _ := context.Server().Context().(*PeerServer)
	commitIndex := context.CommitIndex()

	// Make sure we're not getting a cached value from the registry.
	ps.registry.Invalidate(c.Name)

	// Check if the join command is from a previous peer, who lost all its previous log.
	if peerURL, ok := ps.registry.PeerURL(c.Name); ok {
		// If previous node restarts with different peer URL,
		// update its information.
		if peerURL != c.RaftURL {
			log.Infof("Rejoin with %v instead of %v from %v", c.RaftURL, peerURL, c.Name)
			if err := updatePeerURL(c, ps); err != nil {
				return 0, err
			}
		}
		if c.Name == context.Server().Name() {
			ps.removedInLog = false
		}
		return commitIndex, nil
	}

	// Check if the join command adds an instance that collides with existing one on peer URL.
	peerURLs := ps.registry.PeerURLs(ps.raftServer.Leader(), c.Name)
	for _, peerURL := range peerURLs {
		if peerURL == c.RaftURL {
			log.Warnf("%v tries to join the cluster with existing URL %v", c.Name, c.EtcdURL)
			return 0, etcdErr.NewError(etcdErr.EcodeExistingPeerAddr, c.EtcdURL, context.CommitIndex())
		}
	}

	// Check peer number in the cluster
	count := ps.registry.Count()
	// ClusterConfig doesn't init until first machine is added
	if count > 0 && count >= ps.ClusterConfig().ActiveSize {
		log.Debug("Reject join request from ", c.Name)
		return 0, etcdErr.NewError(etcdErr.EcodeNoMorePeer, "", context.CommitIndex())
	}

	// Add to shared peer registry.
	ps.registry.Register(c.Name, c.RaftURL, c.EtcdURL)

	// Add peer in raft
	if err := context.Server().AddPeer(c.Name, ""); err != nil {
		return 0, err
	}

	// Add peer stats
	if c.Name != ps.RaftServer().Name() {
		ps.followersStats.Followers[c.Name] = &raftFollowerStats{}
		ps.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	if c.Name == context.Server().Name() {
		ps.removedInLog = false
	}
	return commitIndex, nil
}

func updatePeerURL(c *JoinCommand, ps *PeerServer) error {
	log.Debugf("Update peer URL of %v to %v", c.Name, c.RaftURL)
	if err := ps.registry.UpdatePeerURL(c.Name, c.RaftURL); err != nil {
		log.Debugf("Error while updating in registry: %s (%v)", c.Name, err)
		return err
	}
	// Flush commit index, so raft will replay to here when restart
	ps.raftServer.FlushCommitIndex()
	return nil
}
