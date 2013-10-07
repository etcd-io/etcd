/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/test"
	"github.com/coreos/go-etcd/etcd"
)

// Create a single node and try to set value
func TestSingleNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-n=node1", "-f", "-d=/tmp/node1"}

	process, err := os.StartProcess("etcd", args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}
	defer process.Kill()

	time.Sleep(time.Second)

	c := etcd.NewClient()

	c.SyncCluster()
	// Test Set
	result, err := c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL < 95 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.PrevValue != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	// Add a test-and-set test

	// First, we'll test we can change the value if we get it write
	result, match, err := c.TestAndSet("foo", "bar", "foobar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "foobar" || result.PrevValue != "bar" || result.TTL != 99 || !match {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 3 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	// Next, we'll make sure we can't set it without the correct prior value
	_, _, err = c.TestAndSet("foo", "bar", "foofoo", 100)

	if err == nil {
		t.Fatalf("Set 4 expecting error when setting key with incorrect previous value")
	}

	// Finally, we'll make sure a blank previous value still counts as a test-and-set and still has to match
	_, _, err = c.TestAndSet("foo", "", "barbar", 100)

	if err == nil {
		t.Fatalf("Set 5 expecting error when setting key with blank (incorrect) previous value")
	}
}

// TestInternalVersionFail will ensure that etcd does not come up if the internal raft
// versions do not match.
func TestInternalVersionFail(t *testing.T) {
	checkedVersion := false
	testMux := http.NewServeMux()

	testMux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "This is not a version number")
		checkedVersion = true
	})

	testMux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not attempt to join!")
	})

	ts := httptest.NewServer(testMux)
	defer ts.Close()

	fakeURL, _ := url.Parse(ts.URL)

	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-n=node1", "-f", "-d=/tmp/node1", "-vv", "-C=" + fakeURL.Host}

	process, err := os.StartProcess("etcd", args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}
	defer process.Kill()

	time.Sleep(time.Second)

	_, err = http.Get("http://127.0.0.1:4001")

	if err == nil {
		t.Fatal("etcd node should not be up")
		return
	}

	if checkedVersion == false {
		t.Fatal("etcd did not check the version")
		return
	}
}

// This test creates a single node and then set a value to it.
// Then this test kills the node and restart it and tries to get the value again.
func TestSingleNodeRecovery(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-n=node1", "-d=/tmp/node1"}

	process, err := os.StartProcess("etcd", append(args, "-f"), procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}

	time.Sleep(time.Second)

	c := etcd.NewClient()

	c.SyncCluster()
	// Test Set
	result, err := c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL < 95 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	process.Kill()

	process, err = os.StartProcess("etcd", args, procAttr)
	defer process.Kill()
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}

	time.Sleep(time.Second)

	results, err := c.Get("foo")
	if err != nil {
		t.Fatal("get fail: " + err.Error())
		return
	}

	result = results[0]

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL > 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Recovery Get failed with %s %s %v", result.Key, result.Value, result.TTL)
	}
}

// Create a three nodes and try to set value
func templateTestSimpleMultiNode(t *testing.T, tls bool) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3

	_, etcds, err := test.CreateCluster(clusterSize, procAttr, tls)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer test.DestroyCluster(etcds)

	time.Sleep(time.Second)

	c := etcd.NewClient()

	c.SyncCluster()

	// Test Set
	result, err := c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL < 95 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.PrevValue != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

}

func TestSimpleMultiNode(t *testing.T) {
	templateTestSimpleMultiNode(t, false)
}

func TestSimpleMultiNodeTls(t *testing.T) {
	templateTestSimpleMultiNode(t, true)
}

// Create a five nodes
// Kill all the nodes and restart
func TestMultiNodeKillAllAndRecovery(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := test.CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	c := etcd.NewClient()

	c.SyncCluster()

	time.Sleep(time.Second)

	// send 10 commands
	for i := 0; i < 10; i++ {
		// Test Set
		_, err := c.Set("foo", "bar", 0)
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(time.Second)

	// kill all
	test.DestroyCluster(etcds)

	time.Sleep(time.Second)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(time.Second)

	for i := 0; i < clusterSize; i++ {
		etcds[i], err = os.StartProcess("etcd", argGroup[i], procAttr)
	}

	go test.Monitor(clusterSize, 1, leaderChan, all, stop)

	<-all
	<-leaderChan

	result, err := c.Set("foo", "bar", 0)

	if err != nil {
		panic(err)
	}

	if result.Index != 18 {
		t.Fatalf("recovery failed! [%d/18]", result.Index)
	}

	// kill all
	test.DestroyCluster(etcds)
}

// Create a five nodes
// Randomly kill one of the node and keep on sending set command to the cluster
func TestMultiNodeKillOne(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := test.CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer test.DestroyCluster(etcds)

	time.Sleep(2 * time.Second)

	c := etcd.NewClient()

	c.SyncCluster()

	stop := make(chan bool)
	// Test Set
	go test.Set(stop)

	for i := 0; i < 10; i++ {
		num := rand.Int() % clusterSize
		fmt.Println("kill node", num+1)

		// kill
		etcds[num].Kill()
		etcds[num].Release()
		time.Sleep(time.Second)

		// restart
		etcds[num], err = os.StartProcess("etcd", argGroup[num], procAttr)
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Second)
	}
	fmt.Println("stop")
	stop <- true
	<-stop
}

// This test will kill the current leader and wait for the etcd cluster to elect a new leader for 200 times.
// It will print out the election time and the average election time.
func TestKillLeader(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := test.CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer test.DestroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(time.Second)

	go test.Monitor(clusterSize, 1, leaderChan, all, stop)

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
	stop <- true
}

// TestKillRandom kills random machines in the cluster and
// restart them after all other machines agree on the same leader
func TestKillRandom(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 9
	argGroup, etcds, err := test.CreateCluster(clusterSize, procAttr, false)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer test.DestroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(3 * time.Second)

	go test.Monitor(clusterSize, 4, leaderChan, all, stop)

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

		time.Sleep(ElectionTimeout)

		<-leaderChan

		for num, _ := range toKill {
			etcds[num], err = os.StartProcess("etcd", argGroup[num], procAttr)
		}

		toKill = make(map[int]bool)
		<-all
	}

	stop <- true
}

// remove the node and node rejoin with previous log
func TestRemoveNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3
	argGroup, etcds, _ := test.CreateCluster(clusterSize, procAttr, false)

	time.Sleep(time.Second)

	c := etcd.NewClient()

	c.SyncCluster()

	rmReq, _ := http.NewRequest("DELETE", "http://127.0.0.1:7001/remove/node3", nil)

	client := &http.Client{}
	for i := 0; i < 2; i++ {
		for i := 0; i < 2; i++ {
			client.Do(rmReq)

			etcds[2].Wait()

			resp, err := c.Get("_etcd/machines")

			if err != nil {
				panic(err)
			}

			if len(resp) != 2 {
				t.Fatal("cannot remove machine")
			}

			if i == 1 {
				// rejoin with log
				etcds[2], err = os.StartProcess("etcd", argGroup[2], procAttr)
			} else {
				// rejoin without log
				etcds[2], err = os.StartProcess("etcd", append(argGroup[2], "-f"), procAttr)
			}

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second)

			resp, err = c.Get("_etcd/machines")

			if err != nil {
				panic(err)
			}

			if len(resp) != 3 {
				t.Fatal("add machine fails")
			}
		}

		// first kill the node, then remove it, then add it back
		for i := 0; i < 2; i++ {
			etcds[2].Kill()
			etcds[2].Wait()

			client.Do(rmReq)

			resp, err := c.Get("_etcd/machines")

			if err != nil {
				panic(err)
			}

			if len(resp) != 2 {
				t.Fatal("cannot remove machine")
			}

			if i == 1 {
				// rejoin with log
				etcds[2], err = os.StartProcess("etcd", append(argGroup[2]), procAttr)
			} else {
				// rejoin without log
				etcds[2], err = os.StartProcess("etcd", append(argGroup[2], "-f"), procAttr)
			}

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second)

			resp, err = c.Get("_etcd/machines")

			if err != nil {
				panic(err)
			}

			if len(resp) != 3 {
				t.Fatal("add machine fails")
			}
		}
	}
	test.DestroyCluster(etcds)

}

func templateBenchmarkEtcdDirectCall(b *testing.B, tls bool) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3
	_, etcds, _ := test.CreateCluster(clusterSize, procAttr, tls)

	defer test.DestroyCluster(etcds)

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
