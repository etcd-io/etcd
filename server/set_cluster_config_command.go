package server

import (
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
)

func init() {
	raft.RegisterCommand(&SetClusterConfigCommand{})
}

// SetClusterConfigCommand sets the cluster-level configuration.
type SetClusterConfigCommand struct {
	Config *ClusterConfig `json:"config"`
}

// CommandName returns the name of the command.
func (c *SetClusterConfigCommand) CommandName() string {
	return "etcd:setClusterConfig"
}

// Apply updates the cluster configuration.
func (c *SetClusterConfigCommand) Apply(context raft.Context) (interface{}, error) {
	ps, _ := context.Server().Context().(*PeerServer)
	ps.SetClusterConfig(c.Config)
	return nil, nil
}
