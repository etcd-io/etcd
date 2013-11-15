package test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This test will kill the current leader and wait for the etcd cluster to elect a new leader for 200 times.
// It will print out the election time and the average election time.
func TestKillLeader(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)
	if err != nil {
		t.Fatal("cannot create cluster")
	}
	defer DestroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(time.Second)

	go Monitor(clusterSize, 1, leaderChan, all, stop)

	var totalTime time.Duration

	leader := "http://127.0.0.1:7001"

	for i := 0; i < clusterSize; i++ {
		fmt.Println("leader is ", leader)
		port, _ := strconv.Atoi(strings.Split(leader, ":")[2])
		num := port - 7001
		fmt.Println("kill server ", num)
		etcds[num].Kill()
		etcds[num].Release()

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

		etcds[num], err = os.StartProcess(EtcdBinPath, argGroup[num], procAttr)
	}
	stop <- true
}
