package command

import (
	"errors"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewSetDirCommand returns the CLI command for "setDir".
func NewSetDirCommand() cli.Command {
	return cli.Command{
		Name:  "setdir",
		Usage: "create a new or existing directory",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "ttl", Value: 0, Usage: "key time-to-live"},
		},
		Action: func(c *cli.Context) {
			handleDir(c, setDirCommandFunc)
		},
	}
}

// setDirCommandFunc executes the "setDir" command.
func setDirCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	ttl := c.Int("ttl")

	return client.SetDir(key, uint64(ttl))
}
