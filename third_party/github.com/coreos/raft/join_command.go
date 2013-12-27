package raft

// Join command interface
type JoinCommand interface {
	Command
	NodeName() string
}

// Join command
type DefaultJoinCommand struct {
	Name             string `json:"name"`
	ConnectionString string `json:"connectionString"`
}

// The name of the Join command in the log
func (c *DefaultJoinCommand) CommandName() string {
	return "raft:join"
}

func (c *DefaultJoinCommand) Apply(server Server) (interface{}, error) {
	err := server.AddPeer(c.Name, c.ConnectionString)

	return []byte("join"), err
}

func (c *DefaultJoinCommand) NodeName() string {
	return c.Name
}
