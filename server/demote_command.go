package server

import (
	"fmt"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
)

func init() {
	raft.RegisterCommand(&DemoteCommand{})
}

// DemoteCommand represents a command to change a peer to a proxy.
type DemoteCommand struct {
	Name string `json:"name"`
}

// CommandName returns the name of the command.
func (c *DemoteCommand) CommandName() string {
	return "etcd:demote"
}

// Apply executes the command.
func (c *DemoteCommand) Apply(context raft.Context) (interface{}, error) {
	ps, _ := context.Server().Context().(*PeerServer)

	// Ignore this command if there is no peer.
	if !ps.registry.PeerExists(c.Name) {
		return nil, fmt.Errorf("peer does not exist: %s", c.Name)
	}

	// Save URLs.
	clientURL, _ := ps.registry.ClientURL(c.Name)
	peerURL, _ := ps.registry.PeerURL(c.Name)

	// Perform a removal.
	(&RemoveCommand{Name: c.Name}).Apply(context)

	// Register node as a proxy.
	ps.registry.RegisterProxy(c.Name, peerURL, clientURL)

	// Update mode if this change applies to this server.
	if c.Name == ps.Config.Name {
		log.Infof("Set mode after demotion: %s", c.Name)
		ps.setMode(ProxyMode)
	}

	return nil, nil
}

// NodeName returns the name of the affected node.
func (c *DemoteCommand) NodeName() string {
	return c.Name
}
