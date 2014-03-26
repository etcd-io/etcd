package test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

// Create a five nodes
// Kill one of the node and rejoin with different name
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

	c := etcd.NewClient(nil)

	c.SyncCluster()

	num := rand.Int() % clusterSize
	fmt.Println("kill node", num+1)

	// kill
	etcds[num].Kill()
	etcds[num].Release()
	time.Sleep(time.Second)

	// restart
	etcds[num], err = os.StartProcess(EtcdBinPath, argGroup[num], procAttr)
	if err != nil {
		panic(err)
	}

	timer := time.AfterFunc(time.Second, func (){
		etcds[num].Kill()
		etcds[num].Release()
		etcds[num] = nil
		t.Fatal("new etcd should fail immediately")
	})
	etcds[num].Wait()
	timer.Stop()
}
