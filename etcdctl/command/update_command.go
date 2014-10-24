package command

import (
	"errors"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewUpdateCommand returns the CLI command for "update".
func NewUpdateCommand() cli.Command {
	return cli.Command{
		Name:  "update",
		Usage: "update an existing key with a given value",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "ttl", Value: 0, Usage: "key time-to-live"},
		},
		Action: func(c *cli.Context) {
			handleKey(c, updateCommandFunc)
		},
	}
}

// updateCommandFunc executes the "update" command.
func updateCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	value, err := argOrStdin(c.Args(), os.Stdin, 1)
	if err != nil {
		return nil, errors.New("Value required")
	}

	ttl := c.Int("ttl")

	return client.Update(key, value, uint64(ttl))
}
