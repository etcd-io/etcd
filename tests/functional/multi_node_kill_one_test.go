package test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
)

// Create a five nodes
// Randomly kill one of the node and keep on sending set command to the cluster
func TestMultiNodeKillOne(t *testing.T) {
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

	stop := make(chan bool)
	// Test Set
	go Set(stop)

	for i := 0; i < 10; i++ {
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
		time.Sleep(time.Second)
	}
	fmt.Println("stop")
	stop <- true
	<-stop
}
