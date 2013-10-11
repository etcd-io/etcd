package server

import (
	"encoding/binary"
	"fmt"
	"path"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

func init() {
	raft.RegisterCommand(&JoinCommand{})
}

// The JoinCommand adds a node to the cluster.
type JoinCommand struct {
	RaftVersion    string `json:"raftVersion"`
	Name           string `json:"name"`
	RaftURL        string `json:"raftURL"`
	EtcdURL        string `json:"etcdURL"`
	MaxClusterSize int    `json:"maxClusterSize"`
}

func NewJoinCommand(version, name, raftUrl, etcdUrl string, maxClusterSize int) *JoinCommand {
	return &JoinCommand{
		RaftVersion:    version,
		Name:           name,
		RaftURL:        raftUrl,
		EtcdURL:        etcdUrl,
		MaxClusterSize: maxClusterSize,
	}
}

// The name of the join command in the log
func (c *JoinCommand) CommandName() string {
	return "etcd:join"
}

// Join a server to the cluster
func (c *JoinCommand) Apply(server *raft.Server) (interface{}, error) {
	s, _ := server.StateMachine().(*store.Store)
	r, _ := server.Context().(*RaftServer)

	// check if the join command is from a previous machine, who lost all its previous log.
	e, _ := s.Get(path.Join("/_etcd/machines", c.Name), false, false, server.CommitIndex(), server.Term())

	b := make([]byte, 8)
	binary.PutUvarint(b, server.CommitIndex())

	if e != nil {
		return b, nil
	}

	// check machine number in the cluster
	if s.MachineCount() == c.MaxClusterSize {
		log.Debug("Reject join request from ", c.Name)
		return []byte{0}, etcdErr.NewError(etcdErr.EcodeNoMoreMachine, "", server.CommitIndex(), server.Term())
	}

	addNameToURL(c.Name, c.RaftVersion, c.RaftURL, c.EtcdURL)

	// add peer in raft
	err := server.AddPeer(c.Name, "")

	// add machine in etcd storage
	key := path.Join("_etcd/machines", c.Name)
	value := fmt.Sprintf("raft=%s&etcd=%s&raftVersion=%s", c.RaftURL, c.EtcdURL, c.RaftVersion)
	s.Create(key, value, false, false, store.Permanent, server.CommitIndex(), server.Term())

	// add peer stats
	if c.Name != r.Name() {
		r.followersStats.Followers[c.Name] = &raftFollowerStats{}
		r.followersStats.Followers[c.Name].Latency.Minimum = 1 << 63
	}

	return b, err
}

func (c *JoinCommand) NodeName() string {
	return c.Name
}
