package tests

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/etcd/boot"
	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/server"
)

const (
	testName              = "ETCDTEST"
	testClientURL         = "localhost:4401"
	testRaftURL           = "localhost:7701"
	testSnapshotCount     = 10000
	testHeartbeatInterval = 50
	testElectionTimeout   = 200
	testMaxClusterSize    = 9
)

var (
	client = http.Client{
		Transport: &http.Transport{
			Dial: dialTimeoutFast,
		},
	}
)

type Instance struct {
	Conf *config.Config
	Server *server.Server
	firstStart bool
	stop func()
	wait func()
}

func NewInstance() *Instance {
	return &Instance{Conf: config.New(), firstStart: true}
}

// Start starts the instance, and ensures that it is serving
// It will ignore log data if the instance starts at the first time.
func (i *Instance) Start() error {
	if i.firstStart {
		// Remove old data at the first start
		oldForce := i.Conf.Force
		i.Conf.Force = true
		defer func() {
			i.Conf.Force = oldForce
			i.firstStart = false
		}()
	}

	var err error
	i.Server, i.stop, i.wait, _, err = boot.Start(i.Conf)
	if err != nil {
		return err
	}
	// make sure that it is serving now
	if !i.IsServing() {
		return fmt.Errorf("Failed check etcd is serving")
	}
	return nil
}

// Stop stops the instance.
func (i *Instance) Stop() {
	if i.stop != nil {
		i.stop()
		i.stop = nil
		i.wait = nil
	}
}

// Wait waits the instance to stop.
func (i *Instance) Wait() {
	if i.wait != nil {
		i.wait()
		i.stop = nil
		i.wait = nil
	}
}

func (i *Instance) IsServing() bool {
	_, err := client.Get(i.Server.URL() + "/v1/leader")
	if err != nil {
		return false
	}
	return true
}

func (i *Instance) GetLeader() (string, error) {

	resp, err := client.Get(i.Server.URL() + "/v1/leader")

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

// Starts a new server.
func RunServer(f func(*server.Server)) {
	i := NewInstance()
	c := i.Conf

	c.Addr = testClientURL
	c.Name = testName
	c.Peer.Addr = testRaftURL

	c.Peer.HeartbeatInterval = testHeartbeatInterval
	c.Peer.ElectionTimeout = testElectionTimeout
	c.SnapshotCount = testSnapshotCount
	c.MaxClusterSize = testMaxClusterSize
	c.DataDir = "/tmp/node"

	i.Start()

	// Execute the function passed in.
	f(i.Server)

	i.Stop()
}

type Cluster struct {
	Size int
	Instances []*Instance
	client *http.Client
}

// Create a cluster of etcd nodes
func CreateCluster(size int, ssl bool) *Cluster {
	if size <= 0 {
		return nil
	}
	instances := make([]*Instance, size)

	for i := 0; i < size; i++ {
		strI := strconv.Itoa(i + 1)
		instances[i] = NewInstance()
		c := instances[i].Conf
		c.DataDir = "/tmp/node"+strI
		c.Name = "node"+strI
		c.Addr = "127.0.0.1:400"+strI
		c.Peer.Addr = "127.0.0.1:700"+strI
		if i == 0 {
			if ssl {
				c.Peer.CAFile = "../../fixtures/ca/ca.crt"
				c.Peer.CertFile = "../../fixtures/ca/server.crt"
				c.Peer.KeyFile = "../../fixtures/ca/server.key.insecure"
			}
		} else {
			c.Peers = []string{"127.0.0.1:7001"}
			if ssl {
				c.Peer.CAFile = "../../fixtures/ca/ca.crt"
				c.Peer.CertFile = "../../fixtures/ca/server2.crt"
				c.Peer.KeyFile = "../../fixtures/ca/server2.key.insecure"
			}
		}
	}

	return &Cluster{Size: size, Instances: instances}
}

// Start all the nodes in the cluster
func (c *Cluster) Start() bool {
	ok := true
	wg := &sync.WaitGroup{}

	for index, i := range c.Instances {
		// The problem is that if the master isn't up then the children
		// have to retry. This retry can take upwards of 15 seconds
		// which slows tests way down and some of them fail.
		if index == 0 {
			if err := i.Start(); err != nil {
				log.Warn(err)
				ok = false
			}
			continue
		}
		wg.Add(1)
		go func() {
			if err := i.Start(); err != nil {
				log.Warn(err)
				ok = false
			}
			wg.Done()
		}()
	}

	wg.Wait()
	return ok
}

// Stop all the nodes in the cluster
func (c *Cluster) Stop() {
	wg := &sync.WaitGroup{}
	for _, i := range c.Instances {
		wg.Add(1)
		go func() {
			i.Stop()
			wg.Done()
		}()
	}
	wg.Wait()
}

func (c *Cluster) StopOne(index int) {
	c.Instances[index].Stop()
}

func (c *Cluster) StartOne(index int) {
	c.Instances[index].Start()
}

func (c *Cluster) Monitor(allowDeadNum int, leaderChan chan string, all chan bool, stop chan bool) {
	leaderMap := make(map[int]string)

	for {
		knownLeader := "unknown"
		dead := 0
		var i int

		for i = 0; i < c.Size; i++ {
			leader, err := c.Instances[i].GetLeader()

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

		if i == c.Size {
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

// Dial with timeout
func dialTimeoutFast(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Millisecond*10)
}
