package command

import (
	"errors"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewRemoveCommand returns the CLI command for "rmdir".
func NewRemoveDirCommand() cli.Command {
	return cli.Command{
		Name:  "rmdir",
		Usage: "removes the key if it is an empty directory or a key-value pair",
		Action: func(c *cli.Context) {
			handleDir(c, removeDirCommandFunc)
		},
	}
}

// removeDirCommandFunc executes the "rmdir" command.
func removeDirCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]

	return client.DeleteDir(key)
}
