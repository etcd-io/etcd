package command

import (
	"errors"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewMakeCommand returns the CLI command for "mk".
func NewMakeCommand() cli.Command {
	return cli.Command{
		Name:  "mk",
		Usage: "make a new key with a given value",
		Flags: []cli.Flag{
			cli.IntFlag{"ttl", 0, "key time-to-live"},
		},
		Action: func(c *cli.Context) {
			handleKey(c, makeCommandFunc)
		},
	}
}

// makeCommandFunc executes the "make" command.
func makeCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	value, err := argOrStdin(c.Args(), os.Stdin, 1)
	if err != nil {
		return nil, errors.New("Value required")
	}

	ttl := c.Int("ttl")

	return client.Create(key, value, uint64(ttl))
}
