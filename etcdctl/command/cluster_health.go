package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/etcdserver/stats"
)

func NewClusterHealthCommand() cli.Command {
	return cli.Command{
		Name:   "cluster-health",
		Usage:  "check the health of the etcd cluster",
		Flags:  []cli.Flag{},
		Action: handleClusterHealth,
	}
}

func handleClusterHealth(c *cli.Context) {
	endpoints, err := getEndpoints(c)
	if err != nil {
		handleError(ErrorFromEtcd, err)
	}
	tr, err := getTransport(c)
	if err != nil {
		handleError(ErrorFromEtcd, err)
	}

	client := etcd.NewClient(endpoints)
	client.SetTransport(tr)

	if c.GlobalBool("debug") {
		go dumpCURL(client)
	}

	if ok := client.SyncCluster(); !ok {
		handleError(FailedToConnectToHost, errors.New("cannot sync with the cluster using endpoints "+strings.Join(endpoints, ", ")))
	}

	// do we have a leader?
	cl := client.GetCluster()
	ep, ls0, err := getLeaderStats(tr, cl)
	if err != nil {
		fmt.Println("cluster may be unhealthy: failed to connect", cl)
		os.Exit(1)
	}

	// is raft stable and making progress?
	client = etcd.NewClient([]string{ep})
	client.SetTransport(tr)
	resp, err := client.Get("/", false, false)
	if err != nil {
		fmt.Println("cluster is unhealthy")
		os.Exit(1)
	}
	rt0, ri0 := resp.RaftTerm, resp.RaftIndex
	time.Sleep(time.Second)

	resp, err = client.Get("/", false, false)
	if err != nil {
		fmt.Println("cluster is unhealthy")
		os.Exit(1)
	}
	rt1, ri1 := resp.RaftTerm, resp.RaftIndex

	if rt0 != rt1 {
		fmt.Println("cluster is unhealthy")
		os.Exit(1)
	}

	if ri1 == ri0 {
		fmt.Println("cluster is unhealthy")
		os.Exit(1)
	}

	// are all the members makeing progress?
	_, ls1, err := getLeaderStats(tr, []string{ep})
	if err != nil {
		fmt.Println("cluster is unhealthy")
		os.Exit(1)
	}

	fmt.Println("cluster is healthy")
	// self is healthy
	var prints []string

	prints = append(prints, fmt.Sprintf("member %s is healthy\n", ls1.Leader))
	for name, fs0 := range ls0.Followers {
		fs1, ok := ls1.Followers[name]
		if !ok {
			fmt.Println("Cluster configuration changed during health checking. Please retry.")
			os.Exit(1)
		}
		if fs1.Counts.Success <= fs0.Counts.Success {
			prints = append(prints, fmt.Sprintf("member %s is unhealthy\n", name))
		} else {
			prints = append(prints, fmt.Sprintf("member %s is healthy\n", name))
		}
	}

	sort.Strings(prints)
	for _, p := range prints {
		fmt.Print(p)
	}
	os.Exit(0)
}

func getLeaderStats(tr *http.Transport, endpoints []string) (string, *stats.LeaderStats, error) {
	// go-etcd does not support cluster stats, use http client for now
	// TODO: use new etcd client with new member/stats endpoint
	httpclient := http.Client{
		Transport: tr,
	}

	for _, ep := range endpoints {
		resp, err := httpclient.Get(ep + "/v2/stats/leader")
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}

		ls := &stats.LeaderStats{}
		d := json.NewDecoder(resp.Body)
		err = d.Decode(ls)
		if err != nil {
			continue
		}
		return ep, ls, nil
	}
	return "", nil, errors.New("no leader")
}
