package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

var client = http.Client{
	Transport: &http.Transport{
		Dial: dialTimeoutFast,
	},
}

// Sending set commands
func set(stop chan bool) {

	stopSet := false
	i := 0
	c := etcd.NewClient()
	for {
		key := fmt.Sprintf("%s_%v", "foo", i)

		result, err := c.Set(key, "bar", 0)

		if err != nil || result.Key != "/"+key || result.Value != "bar" {
			select {
			case <-stop:
				stopSet = true

			default:
			}
		}

		select {
		case <-stop:
			stopSet = true

		default:
		}

		if stopSet {
			break
		}

		i++
	}
	fmt.Println("set stop")
	stop <- true
}

// Create a cluster of etcd nodes
func createCluster(size int, procAttr *os.ProcAttr) ([][]string, []*os.Process, error) {
	argGroup := make([][]string, size)
	for i := 0; i < size; i++ {
		if i == 0 {
			argGroup[i] = []string{"etcd", "-h=127.0.0.1", "-d=/tmp/node1"}
		} else {
			strI := strconv.Itoa(i + 1)
			argGroup[i] = []string{"etcd", "-h=127.0.0.1", "-c=400" + strI, "-s=700" + strI, "-d=/tmp/node" + strI, "-C=127.0.0.1:7001"}
		}
	}

	etcds := make([]*os.Process, size)

	for i, _ := range etcds {
		var err error
		etcds[i], err = os.StartProcess("etcd", append(argGroup[i], "-f"), procAttr)
		if err != nil {
			return nil, nil, err
		}

		// TODOBP: Change this sleep to wait until the master is up.
		// The problem is that if the master isn't up then the children
		// have to retry. This retry can take upwards of 15 seconds
		// which slows tests way down and some of them fail.
		if i == 0 {
			time.Sleep(time.Second)
		}
	}

	return argGroup, etcds, nil
}

// Destroy all the nodes in the cluster
func destroyCluster(etcds []*os.Process) error {
	for i, etcd := range etcds {
		err := etcd.Kill()
		fmt.Println("kill ", i)
		if err != nil {
			panic(err.Error())
		}
		etcd.Release()
	}
	return nil
}

//
func leaderMonitor(size int, allowDeadNum int, leaderChan chan string) {
	leaderMap := make(map[int]string)
	baseAddrFormat := "http://0.0.0.0:400%d/leader"

	for {
		knownLeader := "unknown"
		dead := 0
		var i int

		for i = 0; i < size; i++ {
			leader, err := getLeader(fmt.Sprintf(baseAddrFormat, i+1))

			if err == nil {
				leaderMap[i] = leader

				if knownLeader == "unknown" {
					knownLeader = leader
				} else {
					if leader != knownLeader {
						break
					}

				}

			} else {
				dead++
				if dead > allowDeadNum {
					break
				}
			}

		}

		if i == size {
			select {
			case <-leaderChan:
				leaderChan <- knownLeader
			default:
				leaderChan <- knownLeader
			}

		}

		time.Sleep(time.Millisecond * 10)
	}

}

func getLeader(addr string) (string, error) {

	resp, err := client.Get(addr)

	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return "", fmt.Errorf("no leader")
	}

	b, err := ioutil.ReadAll(resp.Body)

	resp.Body.Close()

	if err != nil {
		return "", err
	}

	return string(b), nil

}

func directSet() {
	c := make(chan bool, 1000)
	for i := 0; i < 1000; i++ {
		go send(c)
	}

	for i := 0; i < 1000; i++ {
		<-c
	}
}

func send(c chan bool) {
	for i := 0; i < 10; i++ {
		command := &SetCommand{}
		command.Key = "foo"
		command.Value = "bar"
		command.ExpireTime = time.Unix(0, 0)
		raftServer.Do(command)
	}
	c <- true
}

// Dial with timeout
func dialTimeoutFast(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Millisecond*10)
}
