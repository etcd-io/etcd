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
	argGroup, etcds, err := createCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer destroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(time.Second)

	go monitor(clusterSize, 1, leaderChan, all, stop)

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

		fmt.Println("Leader election time is ", take, "with election timeout", ElectionTimeout)
		fmt.Println("Leader election time average is", avgTime, "with election timeout", ElectionTimeout)
		etcds[num], err = os.StartProcess("etcd", argGroup[num], procAttr)
	}
	stop<-true
}

// TestKillRandom kills random machines in the cluster and
// restart them after all other machines agree on the same leader
func TestKillRandom(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 9
	argGroup, etcds, err := createCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer destroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(3 * time.Second)


	go monitor(clusterSize, 4, leaderChan, all, stop)

	toKill := make(map[int]bool)

	for i := 0; i < 200; i++ {
		fmt.Printf("TestKillRandom Round[%d/200]\n", i)

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

		time.Sleep(ElectionTimeout)

		<-leaderChan

		for num, _ := range toKill {
			etcds[num], err = os.StartProcess("etcd", argGroup[num], procAttr)
		}

		toKill = make(map[int]bool)
		<-all
	}

	stop<-true
}

func templateBenchmarkEtcdDirectCall(b *testing.B, tls bool) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3
	_, etcds, _ := createCluster(clusterSize, procAttr, tls)

	defer destroyCluster(etcds)

	time.Sleep(time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := http.Get("http://127.0.0.1:4001/test/speed")
		resp.Body.Close()
	}

}

func BenchmarkEtcdDirectCall(b *testing.B) {
	templateBenchmarkEtcdDirectCall(b, false)
}

func BenchmarkEtcdDirectCallTls(b *testing.B) {
	templateBenchmarkEtcdDirectCall(b, true)
}
