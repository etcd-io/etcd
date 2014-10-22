package command

import (
	"errors"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewUpdateDirCommand returns the CLI command for "updateDir".
func NewUpdateDirCommand() cli.Command {
	return cli.Command{
		Name:  "updatedir",
		Usage: "update an existing directory",
		Flags: []cli.Flag{
			cli.IntFlag{"ttl", 0, "key time-to-live"},
		},
		Action: func(c *cli.Context) {
			handleDir(c, updateDirCommandFunc)
		},
	}
}

// updateDirCommandFunc executes the "updateDir" command.
func updateDirCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	ttl := c.Int("ttl")

	return client.UpdateDir(key, uint64(ttl))
}
