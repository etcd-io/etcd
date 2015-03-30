package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/store"
)

type set struct {
	key   string
	value string
	ttl   int64
}

func NewImportSnapCommand() cli.Command {
	return cli.Command{
		Name:  "import",
		Usage: "import a snapshot to a cluster",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "snap", Value: "", Usage: "Path to the vaild etcd 0.4.x snapshot."},
			cli.IntFlag{Name: "c", Value: 10, Usage: "Number of concurrent clients to import the data"},
		},
		Action: handleImportSnap,
	}
}

func handleImportSnap(c *cli.Context) {
	d, err := ioutil.ReadFile(c.String("snap"))
	if err != nil {
		fmt.Printf("cannot read snapshot file %s\n", c.String("snap"))
		os.Exit(1)
	}

	st := store.New()
	err = st.Recovery(d)
	if err != nil {
		fmt.Printf("cannot recover the snapshot file: %v\n", err)
		os.Exit(1)
	}

	endpoints, err := getEndpoints(c)
	if err != nil {
		handleError(ErrorFromEtcd, err)
	}
	tr, err := getTransport(c)
	if err != nil {
		handleError(ErrorFromEtcd, err)
	}

	wg := &sync.WaitGroup{}
	setc := make(chan set)
	concurrent := c.Int("c")
	fmt.Printf("starting to import snapshot %s with %d clients\n", c.String("snap"), concurrent)
	for i := 0; i < concurrent; i++ {
		client := etcd.NewClient(endpoints)
		client.SetTransport(tr)

		if c.GlobalBool("debug") {
			go dumpCURL(client)
		}

		if ok := client.SyncCluster(); !ok {
			handleError(FailedToConnectToHost, errors.New("cannot sync with the cluster using endpoints "+strings.Join(endpoints, ", ")))
		}
		wg.Add(1)
		go runSet(client, setc, wg)
	}

	all, err := st.Get("/", true, true)
	if err != nil {
		handleError(ErrorFromEtcd, err)
	}
	n := copyKeys(all.Node, setc)
	close(setc)
	fmt.Printf("finished importing %d keys\n", n)
}

func copyKeys(n *store.NodeExtern, setc chan set) int {
	num := 0
	if !n.Dir {
		setc <- set{n.Key, *n.Value, n.TTL}
		return 1
	}
	log.Println("entrying dir:", n.Key)
	for _, nn := range n.Nodes {
		sub := copyKeys(nn, setc)
		num += sub
	}
	return num
}

func runSet(c *etcd.Client, setc chan set, wg *sync.WaitGroup) {
	for s := range setc {
		log.Println("copying key:", s.key)
		if s.ttl != 0 && s.ttl < 300 {
			log.Printf("extending key %s's ttl to 300 seconds", s.key)
			s.ttl = 5 * 60
		}
		_, err := c.Set(s.key, s.value, uint64(s.ttl))
		if err != nil {
			log.Fatalf("failed to copy key: %v\n", err)
		}
	}
	wg.Done()
}
