package command

import (
	"errors"
	"fmt"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewGetCommand returns the CLI command for "get".
func NewGetCommand() cli.Command {
	return cli.Command{
		Name:  "get",
		Usage: "retrieve the value of a key",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "sort", Usage: "returns result in sorted order"},
			cli.BoolFlag{Name: "consistent", Usage: "send request to the leader, thereby guranteeing that any earlier writes will be seen by the read"},
		},
		Action: func(c *cli.Context) {
			handleGet(c, getCommandFunc)
		},
	}
}

// handleGet handles a request that intends to do get-like operations.
func handleGet(c *cli.Context, fn handlerFunc) {
	handlePrint(c, fn, printGet)
}

// printGet writes error message when getting the value of a directory.
func printGet(resp *etcd.Response, format string) {
	if resp.Node.Dir {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("%s: is a directory", resp.Node.Key))
		os.Exit(1)
	}

	printKey(resp, format)
}

// getCommandFunc executes the "get" command.
func getCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	consistent := c.Bool("consistent")
	sorted := c.Bool("sort")

	// Setup consistency on the client.
	if consistent {
		client.SetConsistency(etcd.STRONG_CONSISTENCY)
	} else {
		client.SetConsistency(etcd.WEAK_CONSISTENCY)
	}

	// Retrieve the value from the server.
	return client.Get(key, sorted, false)
}
