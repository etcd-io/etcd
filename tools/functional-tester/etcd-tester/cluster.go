// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	etcdclient "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
)

type cluster struct {
	agentEndpoints []string
	datadir        string

	Size       int
	Agents     []client.Agent
	Stressers  []Stresser
	Names      []string
	ClientURLs []string
}

type ClusterStatus struct {
	AgentStatuses map[string]client.Status
}

// newCluster starts and returns a new cluster. The caller should call Terminate when finished, to shut it down.
func newCluster(agentEndpoints []string, datadir string) (*cluster, error) {
	c := &cluster{
		agentEndpoints: agentEndpoints,
		datadir:        datadir,
	}
	if err := c.Bootstrap(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *cluster) Bootstrap() error {
	size := len(c.agentEndpoints)

	agents := make([]client.Agent, size)
	names := make([]string, size)
	clientURLs := make([]string, size)
	peerURLs := make([]string, size)
	members := make([]string, size)
	for i, u := range c.agentEndpoints {
		var err error
		agents[i], err = client.NewAgent(u)
		if err != nil {
			return err
		}

		names[i] = fmt.Sprintf("etcd-%d", i)

		host, _, err := net.SplitHostPort(u)
		if err != nil {
			return err
		}
		clientURLs[i] = fmt.Sprintf("http://%s:2379", host)
		peerURLs[i] = fmt.Sprintf("http://%s:2380", host)

		members[i] = fmt.Sprintf("%s=%s", names[i], peerURLs[i])
	}
	clusterStr := strings.Join(members, ",")
	token := fmt.Sprint(rand.Int())

	for i, a := range agents {
		_, err := a.Start(
			"-name", names[i],
			"-data-dir", c.datadir,
			"-advertise-client-urls", clientURLs[i],
			"-listen-client-urls", clientURLs[i],
			"-initial-advertise-peer-urls", peerURLs[i],
			"-listen-peer-urls", peerURLs[i],
			"-initial-cluster-token", token,
			"-initial-cluster", clusterStr,
			"-initial-cluster-state", "new",
		)
		if err != nil {
			// cleanup
			for j := 0; j < i; j++ {
				agents[j].Terminate()
			}
			return err
		}
	}

	stressers := make([]Stresser, len(clientURLs))
	for i, u := range clientURLs {
		s := &stresser{
			Endpoint: u,
			// 500000 100B key (50MB)
			KeySize:        100,
			KeySuffixRange: 500000,
			N:              200,
		}
		go s.Stress()
		stressers[i] = s
	}

	c.Size = size
	c.Agents = agents
	c.Stressers = stressers
	c.Names = names
	c.ClientURLs = clientURLs
	return nil
}

func (c *cluster) WaitHealth() error {
	var err error
	for i := 0; i < 10; i++ {
		err = setHealthKey(c.ClientURLs)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}

func (c *cluster) Report() (success, failure int) {
	for _, stress := range c.Stressers {
		s, f := stress.Report()
		success += s
		failure += f
	}
	return
}

func (c *cluster) Cleanup() error {
	var lasterr error
	for _, a := range c.Agents {
		if err := a.Cleanup(); err != nil {
			lasterr = err
		}
	}
	for _, s := range c.Stressers {
		s.Cancel()
	}
	return lasterr
}

func (c *cluster) Terminate() {
	for _, a := range c.Agents {
		a.Terminate()
	}
	for _, s := range c.Stressers {
		s.Cancel()
	}
}

func (c *cluster) Status() ClusterStatus {
	cs := ClusterStatus{
		AgentStatuses: make(map[string]client.Status),
	}

	for i, a := range c.Agents {
		s, err := a.Status()
		// TODO: add a.Desc() as a key of the map
		desc := c.agentEndpoints[i]
		if err != nil {
			cs.AgentStatuses[desc] = client.Status{State: "unknown"}
			log.Printf("etcd-tester: failed to get the status of agent [%s]", desc)
		}
		cs.AgentStatuses[desc] = s
	}
	return cs
}

// setHealthKey sets health key on all given urls.
func setHealthKey(us []string) error {
	for _, u := range us {
		cfg := etcdclient.Config{
			Endpoints: []string{u},
		}
		c, err := etcdclient.New(cfg)
		if err != nil {
			return err
		}
		kapi := etcdclient.NewKeysAPI(c)
		_, err = kapi.Set(context.TODO(), "health", "good", nil)
		if err != nil {
			return err
		}
	}
	return nil
}
