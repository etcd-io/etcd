package command

import (
	"fmt"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

func NewLsCommand() cli.Command {
	return cli.Command{
		Name:  "ls",
		Usage: "retrieve a directory",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "recursive", Usage: "returns all values for key and child keys"},
			cli.BoolFlag{Name: "p", Usage: "append slash (/) to directories"},
		},
		Action: func(c *cli.Context) {
			handleLs(c, lsCommandFunc)
		},
	}
}

// handleLs handles a request that intends to do ls-like operations.
func handleLs(c *cli.Context, fn handlerFunc) {
	handleContextualPrint(c, fn, printLs)
}

// printLs writes a response out in a manner similar to the `ls` command in unix.
// Non-empty directories list their contents and files list their name.
func printLs(c *cli.Context, resp *etcd.Response, format string) {
	if !resp.Node.Dir {
		fmt.Println(resp.Node.Key)
	}
	for _, node := range resp.Node.Nodes {
		rPrint(c, node)
	}
}

// lsCommandFunc executes the "ls" command.
func lsCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	key := "/"
	if len(c.Args()) != 0 {
		key = c.Args()[0]
	}
	recursive := c.Bool("recursive")

	// Retrieve the value from the server.
	return client.Get(key, false, recursive)
}

// rPrint recursively prints out the nodes in the node structure.
func rPrint(c *cli.Context, n *etcd.Node) {

	if n.Dir && c.Bool("p") {
		fmt.Println(fmt.Sprintf("%v/", n.Key))
	} else {
		fmt.Println(n.Key)
	}

	for _, node := range n.Nodes {
		rPrint(c, node)
	}
}
