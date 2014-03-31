package test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"

	etcdtest "github.com/coreos/etcd/tests"
)

// Create a five nodes
// Randomly kill one of the node and keep on sending set command to the cluster
func TestMultiNodeKillOne(t *testing.T) {
	clusterSize := 5
	cluster := etcdtest.NewCluster(clusterSize, false)
	if !cluster.Start() {
		t.Fatal("cannot start cluster")
	}
	defer cluster.Stop()

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
		cluster.StopOne(num)
		time.Sleep(time.Second)

		// restart
		if err := cluster.StartOne(num); err != nil {
			panic(err)
		}
		time.Sleep(time.Second)
	}
	fmt.Println("stop")
	stop <- true
	<-stop
}
