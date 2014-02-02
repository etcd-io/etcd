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

package test

import (
	"fmt"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
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
func Set(stop chan bool) {

	stopSet := false
	i := 0
	c := etcd.NewClient(nil)
	for {
		key := fmt.Sprintf("%s_%v", "foo", i)

		result, err := c.Set(key, "bar", 0)

		if err != nil || result.Node.Key != "/"+key || result.Node.Value != "bar" {
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
	stop <- true
}

// Create a cluster of etcd nodes
func CreateCluster(size int, procAttr *os.ProcAttr, ssl bool) ([][]string, []*os.Process, error) {
	argGroup := make([][]string, size)

	sslServer1 := []string{"-peer-ca-file=../../fixtures/ca/ca.crt",
		"-peer-cert-file=../../fixtures/ca/server.crt",
		"-peer-key-file=../../fixtures/ca/server.key.insecure",
	}

	sslServer2 := []string{"-peer-ca-file=../../fixtures/ca/ca.crt",
		"-peer-cert-file=../../fixtures/ca/server2.crt",
		"-peer-key-file=../../fixtures/ca/server2.key.insecure",
	}

	for i := 0; i < size; i++ {
		if i == 0 {
			argGroup[i] = []string{"etcd", "-data-dir=/tmp/node1", "-name=node1"}
			if ssl {
				argGroup[i] = append(argGroup[i], sslServer1...)
			}
		} else {
			strI := strconv.Itoa(i + 1)
			argGroup[i] = []string{"etcd", "-name=node" + strI, "-addr=127.0.0.1:400" + strI, "-peer-addr=127.0.0.1:700" + strI, "-data-dir=/tmp/node" + strI, "-peers=127.0.0.1:7001"}
			if ssl {
				argGroup[i] = append(argGroup[i], sslServer2...)
			}
		}
	}

	etcds := make([]*os.Process, size)

	for i := range etcds {
		var err error
		etcds[i], err = os.StartProcess(EtcdBinPath, append(argGroup[i], "-f"), procAttr)
		if err != nil {
			return nil, nil, err
		}

		// TODOBP: Change this sleep to wait until the master is up.
		// The problem is that if the master isn't up then the children
		// have to retry. This retry can take upwards of 15 seconds
		// which slows tests way down and some of them fail.
		if i == 0 {
			time.Sleep(time.Second * 2)
		}
	}

	return argGroup, etcds, nil
}

// Destroy all the nodes in the cluster
func DestroyCluster(etcds []*os.Process) error {
	for _, etcd := range etcds {
		err := etcd.Kill()
		if err != nil {
			panic(err.Error())
		}
		etcd.Release()
	}
	return nil
}

//
func Monitor(size int, allowDeadNum int, leaderChan chan string, all chan bool, stop chan bool) {
	leaderMap := make(map[int]string)
	baseAddrFormat := "http://0.0.0.0:400%d"

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
			case <-stop:
				return
			case <-leaderChan:
				leaderChan <- knownLeader
			default:
				leaderChan <- knownLeader
			}

		}
		if dead == 0 {
			select {
			case <-all:
				all <- true
			default:
				all <- true
			}
		}

		time.Sleep(time.Millisecond * 10)
	}

}

func getLeader(addr string) (string, error) {

	resp, err := client.Get(addr + "/v1/leader")

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

// Dial with timeout
func dialTimeoutFast(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Millisecond*10)
}
