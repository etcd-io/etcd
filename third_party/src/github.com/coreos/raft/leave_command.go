package raft

// Leave command interface
type LeaveCommand interface {
	Command
	NodeName() string
}

// Leave command
type DefaultLeaveCommand struct {
	Name string `json:"name"`
}

// The name of the Leave command in the log
func (c *DefaultLeaveCommand) CommandName() string {
	return "raft:leave"
}

func (c *DefaultLeaveCommand) Apply(server Server) (interface{}, error) {
	err := server.RemovePeer(c.Name)

	return []byte("leave"), err
}
func (c *DefaultLeaveCommand) NodeName() string {
	return c.Name
}
