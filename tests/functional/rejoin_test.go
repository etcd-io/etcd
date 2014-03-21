package test

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

func increasePeerAddressPort(args []string, delta int) []string {
	for i, arg := range args {
		if !strings.Contains(arg, "peer-addr") {
			continue
		}
		splitArg := strings.Split(arg, ":")
		port, _ := strconv.Atoi(splitArg[len(splitArg)-1])
		args[i] = "-peer-addr=127.0.0.1:" + strconv.Itoa(port+delta)
		return args
	}
	return append(args, "-peer-addr=127.0.0.1:"+strconv.Itoa(7001+delta))
}

// Create a five-node cluster
// Random kill one of the nodes and restart it with different peer address
func TestRejoinWithDifferentPeerAddress(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer DestroyCluster(etcds)

	time.Sleep(2 * time.Second)

	for i := 0; i < 10; i++ {
		num := rand.Int() % clusterSize
		fmt.Println("kill node", num+1)

		// kill
		etcds[num].Kill()
		etcds[num].Release()
		time.Sleep(time.Second)

		argGroup[num] = increasePeerAddressPort(argGroup[num], clusterSize)
		// restart
		etcds[num], err = os.StartProcess(EtcdBinPath, argGroup[num], procAttr)
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Second)
	}

	c := etcd.NewClient(nil)
	c.SyncCluster()
	result, err := c.Set("foo", "bar", 0)
	if err != nil || result.Node.Key != "/foo" || result.Node.Value != "bar" {
		t.Fatal("Failed to set value in etcd cluster")
	}
}
