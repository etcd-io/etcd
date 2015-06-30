package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/store"
)

type set struct {
	key   string
	value string
	ttl   int64
}

func NewImportSnapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "import a snapshot to a cluster",
		Run:   handleImportSnap,
	}
	cmd.Flags().StringSlice("hidden", []string{}, "Hidden key spaces to import from snapshot")
	cmd.Flags().String("snap", "", "Path to the vaild etcd 0.4.x snapshot.")
	cmd.Flags().Int("c", 10, "Number of concurrent clients to import the data")
	return cmd
}

func handleImportSnap(cmd *cobra.Command, _ []string) {
	snap, _ := cmd.Flags().GetString("snap")
	d, err := ioutil.ReadFile(snap)
	if err != nil {
		if snap == "" {
			fmt.Printf("no snapshot file provided (use --snap)\n")
		} else {
			fmt.Printf("cannot read snapshot file %s\n", snap)
		}
		os.Exit(1)
	}

	st := store.New()
	err = st.Recovery(d)
	if err != nil {
		fmt.Printf("cannot recover the snapshot file: %v\n", err)
		os.Exit(1)
	}

	endpoints, err := getEndpoints(cmd)
	if err != nil {
		handleError(ExitServerError, err)
	}
	tr, err := getTransport(cmd)
	if err != nil {
		handleError(ExitServerError, err)
	}

	wg := &sync.WaitGroup{}
	setc := make(chan set)
	concurrent, _ := cmd.Flags().GetInt("c")
	fmt.Printf("starting to import snapshot %s with %d clients\n", snap, concurrent)
	for i := 0; i < concurrent; i++ {
		client := etcd.NewClient(endpoints)
		client.SetTransport(tr)

		if b, _ := cmd.Flags().GetBool("debug"); b {
			go dumpCURL(client)
		}

		if ok := client.SyncCluster(); !ok {
			handleError(ExitBadConnection, errors.New("cannot sync with the cluster using endpoints "+strings.Join(endpoints, ", ")))
		}
		wg.Add(1)
		go runSet(client, setc, wg)
	}

	all, err := st.Get("/", true, true)
	if err != nil {
		handleError(ExitServerError, err)
	}
	n := copyKeys(all.Node, setc)

	hiddens, _ := cmd.Flags().GetStringSlice("hidden")
	for _, h := range hiddens {
		allh, err := st.Get(h, true, true)
		if err != nil {
			handleError(ExitServerError, err)
		}
		n += copyKeys(allh.Node, setc)
	}
	close(setc)
	wg.Wait()
	fmt.Printf("finished importing %d keys\n", n)
}

func copyKeys(n *store.NodeExtern, setc chan set) int {
	num := 0
	if !n.Dir {
		setc <- set{n.Key, *n.Value, n.TTL}
		return 1
	}
	log.Println("entering dir:", n.Key)
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
