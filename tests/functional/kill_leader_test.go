package test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	etcdtest "github.com/coreos/etcd/tests"
)

// This test will kill the current leader and wait for the etcd cluster to elect a new leader for 200 times.
// It will print out the election time and the average election time.
func TestKillLeader(t *testing.T) {
	clusterSize := 5
	cluster := etcdtest.NewCluster(clusterSize, false)
	if !cluster.Start() {
		t.Fatal("cannot start cluster")
	}
	defer cluster.Stop()

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(time.Second)

	go cluster.Monitor(1, leaderChan, all, stop)

	var totalTime time.Duration

	leader := "http://127.0.0.1:7001"

	for i := 0; i < clusterSize; i++ {
		fmt.Println("leader is ", leader)
		port, _ := strconv.Atoi(strings.Split(leader, ":")[2])
		num := port - 7001
		fmt.Println("kill server ", num)
		cluster.StopOne(num)

		start := time.Now()
		for {
			newLeader := <-leaderChan
			if newLeader != leader {
				leader = newLeader
				break
			}
		}
		take := time.Now().Sub(start)

		totalTime += take
		avgTime := totalTime / (time.Duration)(i+1)
		fmt.Println("Total time:", totalTime, "; Avg time:", avgTime)

		cluster.StartOne(num)
	}
	stop <- true
}
