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

func increaseAddressPort(args []string, delta int) []string {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-addr") && !strings.HasPrefix(arg, "--addr") {
			continue
		}
		splitArg := strings.Split(arg, ":")
		port, _ := strconv.Atoi(splitArg[len(splitArg)-1])
		args[i] = "-addr=127.0.0.1:" + strconv.Itoa(port+delta)
		return args
	}
	return append(args, "-addr=127.0.0.1:"+strconv.Itoa(4001+delta))
}

func increaseDataDir(args []string, delta int) []string {
	for i, arg := range args {
		if !strings.Contains(arg, "-data-dir") {
			continue
		}
		splitArg := strings.Split(arg, "node")
		idx, _ := strconv.Atoi(splitArg[len(splitArg)-1])
		args[i] = "-data-dir=/tmp/node" + strconv.Itoa(idx+delta)
		return args
	}
	return args
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

// Create a five-node cluster
// Replace one of the nodes with different peer address
func TestReplaceWithDifferentPeerAddress(t *testing.T) {
	// TODO(yichengq): find some way to avoid the error that will be
	// caused if some node joins the cluster with the collided name.
	// Possible solutions:
	// 1. Remove itself when executing a join command with the same name
	// and different peer address. However, it should find some way to
	// trigger that execution because the leader may update its address
	// and stop heartbeat.
	// 2. Remove the node with the same name before join each time.
	// But this way could be rather overkill.
	t.Skip("Unimplemented functionality")

	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer DestroyCluster(etcds)

	time.Sleep(2 * time.Second)

	rand.Int()
	for i := 0; i < 10; i++ {
		num := rand.Int() % clusterSize
		fmt.Println("replace node", num+1)

		argGroup[num] = increasePeerAddressPort(argGroup[num], clusterSize)
		argGroup[num] = increaseAddressPort(argGroup[num], clusterSize)
		argGroup[num] = increaseDataDir(argGroup[num], clusterSize)
		// restart
		newEtcd, err := os.StartProcess(EtcdBinPath, append(argGroup[num], "-f"), procAttr)
		if err != nil {
			panic(err)
		}

		etcds[num].Wait()
		etcds[num] = newEtcd
	}

	c := etcd.NewClient(nil)
	c.SyncCluster()
	result, err := c.Set("foo", "bar", 0)
	if err != nil || result.Node.Key != "/foo" || result.Node.Value != "bar" {
		t.Fatal("Failed to set value in etcd cluster")
	}
}

// Create a five-node cluster
// Let the sixth instance join with different name and existing peer address
func TestRejoinWithDifferentName(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer DestroyCluster(etcds)

	time.Sleep(2 * time.Second)

	num := rand.Int() % clusterSize
	fmt.Println("join node 6 that collides with node", num+1)

	// kill
	etcds[num].Kill()
	etcds[num].Release()
	time.Sleep(time.Second)

	for i := 0; i < 2; i++ {
		// restart
		if i == 0 {
			etcds[num], err = os.StartProcess(EtcdBinPath, append(argGroup[num], "-name=node6", "-peers=127.0.0.1:7002"), procAttr)
		} else {
			etcds[num], err = os.StartProcess(EtcdBinPath, append(argGroup[num], "-f", "-name=node6", "-peers=127.0.0.1:7002"), procAttr)
		}
		if err != nil {
			t.Fatal("failed to start process:", err)
		}

		timer := time.AfterFunc(10*time.Second, func() {
			t.Fatal("new etcd should fail immediately")
		})
		etcds[num].Wait()
		etcds[num] = nil
		timer.Stop()
	}
}
