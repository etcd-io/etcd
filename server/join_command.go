package server

import (
	"encoding/binary"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/go-raft"
)

func init() {
	raft.RegisterCommand(&JoinCommand{})
}

// The JoinCommand adds a node to the cluster.
type JoinCommand struct {
	RaftVersion string `json:"raftVersion"`
	Name        string `json:"name"`
	RaftURL     string `json:"raftURL"`
	EtcdURL     string `json:"etcdURL"`
}

func NewJoinCommand(version, name, raftUrl, etcdUrl string) *JoinCommand {
	return &JoinCommand{
		RaftVersion: version,
		Name:        name,
		RaftURL:     raftUrl,
		EtcdURL:     etcdUrl,
	}
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return "etcd:join"
}

// Join a server to the cluster
func (c *JoinCommand) Apply(server *raft.Server) (interface{}, error) {
	ps, _ := server.Context().(*PeerServer)

	b := make([]byte, 8)
	binary.PutUvarint(b, server.CommitIndex())

	// Check if the join command is from a previous machine, who lost all its previous log.
	if _, ok := ps.registry.URL(c.Name); ok {
		return b, nil
	}

	// Check machine number in the cluster
	if ps.registry.Count() == ps.MaxClusterSize {
		log.Debug("Reject join request from ", c.Name)
		return []byte{0}, etcdErr.NewError(etcdErr.EcodeNoMoreMachine, "", server.CommitIndex(), server.Term())
	}

	// Add to shared machine registry.
	ps.registry.Register(c.Name, c.RaftVersion, c.RaftURL, c.EtcdURL, server.CommitIndex(), server.Term())

	// Add peer in raft
	err := server.AddPeer(c.Name, "")

	// Add peer stats
	if c.Name != ps.Name() {
		ps.followersStats.Followers[c.Name] = &raftFollowerStats{}
		ps.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	return b, err
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}
