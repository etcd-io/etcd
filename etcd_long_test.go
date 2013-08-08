package main

import (
	"fmt"
	"math/rand"
	"net/http"
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

	leader := "127.0.0.1:7001"

	for i := 0; i < 10; i++ {
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

// TestKillRandom kills random machines in the cluster and
// restart them after all other machines agree on the same leader
func TestKillRandom(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 9
	argGroup, etcds, err := createCluster(clusterSize, procAttr)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer destroyCluster(etcds)

	leaderChan := make(chan string, 1)

	time.Sleep(3 * time.Second)

	go leaderMonitor(clusterSize, 4, leaderChan)

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

		for num, _ := range toKill {
			etcds[num].Kill()
			etcds[num].Release()
		}

		<-leaderChan

		for num, _ := range toKill {
			etcds[num], err = os.StartProcess("etcd", argGroup[num], procAttr)
		}

		toKill = make(map[int]bool)
	}

	<-leaderChan

}

func BenchmarkEtcdDirectCall(b *testing.B) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3
	_, etcds, _ := createCluster(clusterSize, procAttr)

	defer destroyCluster(etcds)

	time.Sleep(time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := http.Get("http://127.0.0.1:4001/test/speed")
		resp.Body.Close()
	}

}
