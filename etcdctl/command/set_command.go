package command

import (
	"errors"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewSetCommand returns the CLI command for "set".
func NewSetCommand() cli.Command {
	return cli.Command{
		Name:  "set",
		Usage: "set the value of a key",
		Flags: []cli.Flag{
			cli.IntFlag{"ttl", 0, "key time-to-live"},
			cli.StringFlag{"swap-with-value", "", "previous value"},
			cli.IntFlag{"swap-with-index", 0, "previous index"},
		},
		Action: func(c *cli.Context) {
			handleKey(c, setCommandFunc)
		},
	}
}

// setCommandFunc executes the "set" command.
func setCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	value, err := argOrStdin(c.Args(), os.Stdin, 1)
	if err != nil {
		return nil, errors.New("Value required")
	}

	ttl := c.Int("ttl")
	prevValue := c.String("swap-with-value")
	prevIndex := c.Int("swap-with-index")

	if prevValue == "" && prevIndex == 0 {
		return client.Set(key, value, uint64(ttl))
	} else {
		return client.CompareAndSwap(key, value, uint64(ttl), prevValue, uint64(prevIndex))
	}
}
