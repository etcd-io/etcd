package test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

// TestKillRandom kills random peers in the cluster and
// restart them after all other peers agree on the same leader
func TestKillRandom(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 9
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer DestroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(3 * time.Second)

	go Monitor(clusterSize, 4, leaderChan, all, stop)

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
			err := etcds[num].Kill()
			if err != nil {
				panic(err)
			}
			etcds[num].Wait()
		}

		time.Sleep(1 * time.Second)

		<-leaderChan

		for num, _ := range toKill {
			etcds[num], err = os.StartProcess(EtcdBinPath, argGroup[num], procAttr)
		}

		toKill = make(map[int]bool)
		<-all
	}

	stop <- true
}
