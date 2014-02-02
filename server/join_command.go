package server

import (
	"encoding/binary"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&JoinCommand{})
}

// The JoinCommand adds a node to the cluster.
type JoinCommand struct {
	MinVersion	int	`json:"minVersion"`
	MaxVersion	int	`json:"maxVersion"`
	Name		string	`json:"name"`
	RaftURL		string	`json:"raftURL"`
	EtcdURL		string	`json:"etcdURL"`
}

func NewJoinCommand(minVersion int, maxVersion int, name, raftUrl, etcdUrl string) *JoinCommand {
	return &JoinCommand{
		MinVersion:	minVersion,
		MaxVersion:	maxVersion,
		Name:		name,
		RaftURL:	raftUrl,
		EtcdURL:	etcdUrl,
	}
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return "etcd:join"
}

// Join a server to the cluster
func (c *JoinCommand) Apply(context raft.Context) (interface{}, error) {
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
	if ps.registry.Count() == ps.Config.MaxClusterSize {
		log.Debug("Reject join request from ", c.Name)
		return []byte{0}, etcdErr.NewError(etcdErr.EcodeNoMorePeer, "", context.CommitIndex())
	}

	// Add to shared peer registry.
	ps.registry.Register(c.Name, c.RaftURL, c.EtcdURL)

	// Add peer in raft
	err := context.Server().AddPeer(c.Name, "")

	// Add peer stats
	if c.Name != ps.RaftServer().Name() {
		ps.followersStats.Followers[c.Name] = &raftFollowerStats{}
		ps.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	return b, err
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}
