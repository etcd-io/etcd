package main

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
	argGroup, etcds, err := createCluster(clusterSize, procAttr)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer destroyCluster(etcds)

	leaderChan := make(chan string, 1)

	time.Sleep(time.Second)

	go leaderMonitor(clusterSize, 1, leaderChan)

	var totalTime time.Duration

	leader := "0.0.0.0:7001"

	for i := 0; i < 200; i++ {
		port, _ := strconv.Atoi(strings.Split(leader, ":")[1])
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

		fmt.Println("Leader election time is ", take, "with election timeout", ELECTIONTIMEOUT)
		fmt.Println("Leader election time average is", avgTime, "with election timeout", ELECTIONTIMEOUT)
		etcds[num], err = os.StartProcess("etcd", argGroup[num], procAttr)
	}

}
