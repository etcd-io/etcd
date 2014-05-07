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

// Apply attempts to join a machine to the cluster.
func (c *JoinCommandV1) Apply(context raft.Context) (interface{}, error) {
	c2 := &JoinCommandV2{
		MinVersion: c.MinVersion,
		MaxVersion: c.MaxVersion,
		Name:       c.Name,
		PeerURL:    c.RaftURL,
		ClientURL:  c.EtcdURL,
	}
	resp, err := applyJoin(c2, context)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 8)
	binary.PutUvarint(b, resp.CommitIndex)
	return b, nil
}

func (c *JoinCommandV1) NodeName() string {
	return c.Name
}

// JoinCommandV2 represents a request to join the cluster.
// The command returns the joinResponseV2.
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
	resp, err := applyJoin(c, context)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(resp)
	return b, nil
}

func (c *JoinCommandV2) NodeName() string {
	return c.Name
}

type joinResponseV2 struct {
	CommitIndex uint64 `json:"commitIndex"`
}

// applyJoin attempts to join a machine to the cluster.
func applyJoin(c *JoinCommandV2, context raft.Context) (*joinResponseV2, error) {
	ps, _ := context.Server().Context().(*PeerServer)
	msg := &joinResponseV2{context.CommitIndex()}

	// Make sure we're not getting a cached value from the registry.
	ps.registry.Invalidate(c.Name)

	// Check if the join command is from a previous peer, who lost all its previous log.
	if peerURL, ok := ps.registry.PeerURL(c.Name); ok {
		// If previous node restarts with different peer URL,
		// update its information.
		if peerURL != c.PeerURL {
			log.Infof("Rejoin with %v instead of %v from %v", c.PeerURL, peerURL, c.Name)
			if err := updatePeerURL(c, ps); err != nil {
				return nil, err
			}
		}
		if c.Name == context.Server().Name() {
			ps.standbyModeInLog = false
		}
		return msg, nil
	}

	// Check if the join command adds an instance that collides with existing one on peer URL.
	peerURLs := ps.registry.PeerURLs(ps.raftServer.Leader(), c.Name)
	for _, peerURL := range peerURLs {
		if peerURL == c.PeerURL {
			log.Warnf("%v tries to join the cluster with existing URL %v", c.Name, c.ClientURL)
			return nil, etcdErr.NewError(etcdErr.EcodeExistingPeerAddr, c.ClientURL, context.CommitIndex())
		}
	}

	// Check peer number in the cluster
	count := ps.registry.Count()
	// ClusterConfig doesn't init until first machine is added
	if count > 0 && count >= ps.ClusterConfig().ActiveSize {
		log.Debug("Reject join request from ", c.Name)
		return nil, etcdErr.NewError(etcdErr.EcodeNoMorePeer, "", context.CommitIndex())
	}

	// Add to shared peer registry.
	ps.registry.Register(c.Name, c.PeerURL, c.ClientURL)

	// Add peer in raft
	if err := context.Server().AddPeer(c.Name, ""); err != nil {
		return nil, err
	}

	// Add peer stats
	if c.Name != ps.RaftServer().Name() {
		ps.followersStats.Followers[c.Name] = &raftFollowerStats{}
		ps.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	if c.Name == context.Server().Name() {
		ps.standbyModeInLog = false
	}
	return msg, nil
}

func updatePeerURL(c *JoinCommandV2, ps *PeerServer) error {
	log.Debugf("Update peer URL of %v to %v", c.Name, c.PeerURL)
	if err := ps.registry.UpdatePeerURL(c.Name, c.PeerURL); err != nil {
		log.Debugf("Error while updating in registry: %s (%v)", c.Name, err)
		return err
	}
	// Flush commit index, so raft will replay to here when restart
	ps.raftServer.FlushCommitIndex()
	return nil
}
