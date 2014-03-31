package test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	etcdtest "github.com/coreos/etcd/tests"
)

// TestKillRandom kills random peers in the cluster and
// restart them after all other peers agree on the same leader
func TestKillRandom(t *testing.T) {
	clusterSize := 9
	cluster := etcdtest.NewCluster(clusterSize, false)
	if !cluster.Start() {
		t.Fatal("cannot start cluster")
	}
	defer cluster.Stop()

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(3 * time.Second)

	go cluster.Monitor(4, leaderChan, all, stop)

	toKill := make(map[int]bool)

	for i := 0; i < 20; i++ {
		fmt.Printf("TestKillRandom Round[%d/20]\n", i)

		j := 0
		for {

			r := rand.Int31n(9)
			if _, ok := toKill[int(r)]; !ok {
				j++
				toKill[int(r)] = true
			}

			if j > 3 {
				break
			}

		}

		for num := range toKill {
			cluster.StopOne(num)
		}

		time.Sleep(1 * time.Second)

		<-leaderChan

		for num := range toKill {
			cluster.StartOne(num)
		}

		toKill = make(map[int]bool)
		<-all
	}

	stop <- true
}
