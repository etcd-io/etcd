package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"

	"github.com/coreos/etcdctl/third_party/github.com/codegangsta/cli"
	"github.com/coreos/etcdctl/third_party/github.com/coreos/go-etcd/etcd"
)

// NewExportCommand returns the CLI command for "export"
func NewExportCommand() cli.Command {
	return cli.Command{
		Name:  "export",
		Usage: "export the etcd tree in JSON format",
		Description: "The command accepts a single argument, which is the key from which to export. The default is /",
		Action: func(c *cli.Context) {
			handleExport(c, exportCommandFunc)
		},
	}
}

// handleExport gets a directory structure and exports it as JSON
func handleExport(c *cli.Context, fn handlerFunc) {
	response, err := rawhandle(c, fn)
	if err != nil {
		handleError(ErrorFromEtcd, err)
	}
	container := readTree(response.Node.Nodes)
	data := []byte{}
	buf := bytes.NewBuffer(data)
	enc := json.NewEncoder(buf)
	enc.Encode(container)
	fmt.Println(buf.String())
}


// exportCommandFunc fetches the structure under the provided key
func exportCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	base_key := "/"
	if len(c.Args()) != 0 {
		if len(c.Args()) > 1 {
			handleError(MalformedEtcdctlArguments, ErrNumArgs)
		}
		base_key = c.Args()[0]
	}
	response, err := client.Get(base_key, false, true)
	return response, err
}

// readTree builds a map of keys and values. If a node is a directory the function calls itself again to act on the node
func readTree(nodes etcd.Nodes) map[string]interface{} {
	container := make(map[string]interface{})
	for _, node := range nodes {
		key := path.Base(node.Key)
		if node.Dir {
			container[key] = readTree(node.Nodes)
		} else {
			container[key] = node.Value
		}
	}
	return container
}
