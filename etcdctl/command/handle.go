package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

type handlerFunc func(*cli.Context, *etcd.Client) (*etcd.Response, error)
type printFunc func(*etcd.Response, string)

// dumpCURL blindly dumps all curl output to os.Stderr
func dumpCURL(client *etcd.Client) {
	client.OpenCURL()
	for {
		fmt.Fprintf(os.Stderr, "Curl-Example: %s\n", client.RecvCURL())
	}
}

// createHttpPath attaches http scheme to the given address if needed
func createHttpPath(addr string) (string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}

	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u.String(), nil
}

// rawhandle wraps the command function handlers and sets up the
// environment but performs no output formatting.
func rawhandle(c *cli.Context, fn handlerFunc) (*etcd.Response, error) {
	sync := !c.GlobalBool("no-sync")

	peerstr := c.GlobalString("peers")

	// Use an environment variable if nothing was supplied on the
	// command line
	if peerstr == "" {
		peerstr = os.Getenv("ETCDCTL_PEERS")
	}

	// If we still don't have peers, use a default
	if peerstr == "" {
		peerstr = "127.0.0.1:4001"
	}

	peers := strings.Split(peerstr, ",")

	// If no sync, create http path for each peer address
	if !sync {
		revisedPeers := make([]string, 0)
		for _, peer := range peers {
			if revisedPeer, err := createHttpPath(peer); err != nil {
				fmt.Fprintf(os.Stderr, "Unsupported url %v: %v\n", peer, err)
			} else {
				revisedPeers = append(revisedPeers, revisedPeer)
			}
		}
		peers = revisedPeers
	}

	client := etcd.NewClient(peers)

	if c.GlobalBool("debug") {
		go dumpCURL(client)
	}

	// Sync cluster.
	if sync {
		if ok := client.SyncCluster(); !ok {
			handleError(FailedToConnectToHost, errors.New("Cannot sync with the cluster using peers "+strings.Join(peers, ", ")))
		}
	}

	if c.GlobalBool("debug") {
		fmt.Fprintf(os.Stderr, "Cluster-Peers: %s\n",
			strings.Join(client.GetCluster(), " "))
	}

	// Execute handler function.
	return fn(c, client)
}

// handlePrint wraps the command function handlers to parse global flags
// into a client and to properly format the response objects.
func handlePrint(c *cli.Context, fn handlerFunc, pFn printFunc) {
	resp, err := rawhandle(c, fn)

	// Print error and exit, if necessary.
	if err != nil {
		handleError(ErrorFromEtcd, err)
	}

	if resp != nil && pFn != nil {
		pFn(resp, c.GlobalString("output"))
	}
}

// handleDir handles a request that wants to do operations on a single dir.
// Dir cannot be printed out, so we set NIL print function here.
func handleDir(c *cli.Context, fn handlerFunc) {
	handlePrint(c, fn, nil)
}

// handleKey handles a request that wants to do operations on a single key.
func handleKey(c *cli.Context, fn handlerFunc) {
	handlePrint(c, fn, printKey)
}

func handleAll(c *cli.Context, fn handlerFunc) {
	handlePrint(c, fn, printAll)
}

// printKey writes the etcd response to STDOUT in the given format.
func printKey(resp *etcd.Response, format string) {
	// printKey is only for keys, error on directories
	if resp.Node.Dir == true {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("Cannot print key [%s: Is a directory]", resp.Node.Key))
		os.Exit(1)
	}
	printKeyOnly(resp, format)
}

// printAll prints the etcd response in the given format in its best efforts.
func printAll(resp *etcd.Response, format string) {
	if resp.Node.Dir == true {
		return
	}
	printKeyOnly(resp, format)
}

// printKeyOnly only supports to print key correctly.
func printKeyOnly(resp *etcd.Response, format string) {
	// Format the result.
	switch format {
	case "simple":
		fmt.Println(resp.Node.Value)
	case "extended":
		// Extended prints in a rfc2822 style format
		fmt.Println("Key:", resp.Node.Key)
		fmt.Println("Created-Index:", resp.Node.CreatedIndex)
		fmt.Println("Modified-Index:", resp.Node.ModifiedIndex)

		if resp.PrevNode != nil {
			fmt.Println("PrevNode.Value:", resp.PrevNode.Value)
		}

		fmt.Println("TTL:", resp.Node.TTL)
		fmt.Println("Etcd-Index:", resp.EtcdIndex)
		fmt.Println("Raft-Index:", resp.RaftIndex)
		fmt.Println("Raft-Term:", resp.RaftTerm)
		fmt.Println("")
		fmt.Println(resp.Node.Value)
	case "json":
		b, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(b))
	default:
		fmt.Fprintln(os.Stderr, "Unsupported output format:", format)
	}
}
