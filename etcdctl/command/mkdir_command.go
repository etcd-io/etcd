package command

import (
	"errors"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewMakeDirCommand returns the CLI command for "mkdir".
func NewMakeDirCommand() cli.Command {
	return cli.Command{
		Name:  "mkdir",
		Usage: "make a new directory",
		Flags: []cli.Flag{
			cli.IntFlag{"ttl", 0, "key time-to-live"},
		},
		Action: func(c *cli.Context) {
			handleDir(c, makeDirCommandFunc)
		},
	}
}

// makeDirCommandFunc executes the "mkdir" command.
func makeDirCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	ttl := c.Int("ttl")

	return client.CreateDir(key, uint64(ttl))
}
