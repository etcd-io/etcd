package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/etcdctl/third_party/github.com/codegangsta/cli"
	"github.com/coreos/etcdctl/third_party/github.com/coreos/go-etcd/etcd"
)

func NewCopyCommand() cli.Command {
  return cli.Command{
    Name:  "cp",
    Usage: "Copy source keys to destination",

    Flags: []cli.Flag{
      cli.BoolFlag{"recursive, r", "Copy all child directories and keys"},
			cli.BoolFlag{"consistent, c", "Send request to the leader, thereby guranteeing that any earlier writes will be seen by the read"},
    },

		Action: func(c *cli.Context) {
			handleKey(c, copyCommandFunc)
		},
  }
}

// Create the node from the src to dest, the srcKey and destKey allow us
// to know what the destination path should be.
func copy(client *etcd.Client, node *etcd.Node, srcKey, destKey string) {
	dest := strings.Replace(node.Key, srcKey, destKey, 1)
	if node.Dir {
		client.CreateDir(dest, uint64(node.TTL))
	} else {
		client.Create(dest, node.Value, uint64(node.TTL))
	}
}

func rCopy(client *etcd.Client, root *etcd.Node, srcKey, destKey string) {
	copy(client, root, srcKey, destKey)
	for _, node := range root.Nodes {
		rCopy(client, &node, srcKey, destKey)
	}
}

func copyCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {

	if len(c.Args()) == 0 {
		return nil, errors.New("Source key required")
	} else if len(c.Args()) == 1 {
		return nil, errors.New("Destination key required")
	}

	sourceKey := c.Args()[0]
	destKey := c.Args()[1]

	// Setup consistency on the client.
	consistent := c.Bool("consistent")
	if consistent {
		client.SetConsistency(etcd.STRONG_CONSISTENCY)
	} else {
		client.SetConsistency(etcd.WEAK_CONSISTENCY)
	}

	// Check that the destination key doesn't exist
	_, err := client.Get(destKey, false, false)
	if err == nil {
		return nil, fmt.Errorf("Destination key %q already exists", destKey)
	} else {
		etcderr := err.(*etcd.EtcdError)
		if etcderr.ErrorCode != 100 {
			return nil, err
		}
	}

	recursive := c.Bool("recursive")
	resp, err := client.Get(sourceKey, false, recursive)
	if err != nil {
		return nil, err
	}

	rCopy(client, resp.Node, sourceKey, destKey)
	return nil, nil
}
