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

package tester

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"

	clientv2 "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
)

const peerURLPort = 2380

type cluster struct {
	v2 bool // to be deprecated

	agentEndpoints []string
	datadir        string

	stressKeySize        int
	stressKeySuffixRange int

	Size       int
	Agents     []client.Agent
	Stressers  []Stresser
	Names      []string
	GRPCURLs   []string
	ClientURLs []string
}

type ClusterStatus struct {
	AgentStatuses map[string]client.Status
}

// newCluster starts and returns a new cluster. The caller should call Terminate when finished, to shut it down.
func newCluster(cmdFlag flag) (*cluster, error) {
	c := &cluster{
		v2:                   cmdFlag.V2,
		agentEndpoints:       cmdFlag.AgentEndpoints,
		datadir:              cmdFlag.DataDir,
		stressKeySize:        cmdFlag.StressKeySize,
		stressKeySuffixRange: cmdFlag.StressKeySuffixRange,
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
	grpcURLs := make([]string, size)
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

		url, err := url.Parse(u)
		if err != nil {
			return err
		}
		host, _, err := net.SplitHostPort(url.Host)
		if err != nil {
			return err
		}
		grpcURLs[i] = fmt.Sprintf("%s:2378", host)
		clientURLs[i] = fmt.Sprintf("http://%s:2379", host)
		peerURLs[i] = fmt.Sprintf("http://%s:%d", host, peerURLPort)

		members[i] = fmt.Sprintf("%s=%s", names[i], peerURLs[i])
	}
	clusterStr := strings.Join(members, ",")
	token := fmt.Sprint(rand.Int())

	for i, a := range agents {
		flags := []string{
			"--name", names[i],
			"--data-dir", c.datadir,

			"--listen-client-urls", clientURLs[i],
			"--advertise-client-urls", clientURLs[i],

			"--listen-peer-urls", peerURLs[i],
			"--initial-advertise-peer-urls", peerURLs[i],

			"--initial-cluster-token", token,
			"--initial-cluster", clusterStr,
			"--initial-cluster-state", "new",
		}
		if !c.v2 {
			flags = append(flags,
				"--experimental-v3demo",
				"--experimental-gRPC-addr", grpcURLs[i],
			)
		}

		if _, err := a.Start(flags...); err != nil {
			// cleanup
			for j := 0; j < i; j++ {
				agents[j].Terminate()
			}
			return err
		}
	}

	var stressers []Stresser
	if c.v2 {
		for _, u := range clientURLs {
			s := &stresserV2{
				Endpoint:       u,
				KeySize:        c.stressKeySize,
				KeySuffixRange: c.stressKeySuffixRange,
				N:              200,
			}
			go s.Stress()
			stressers = append(stressers, s)
		}
	}

	// v3
	s := &stresser{
		Endpoints:      grpcURLs,
		KeySize:        c.stressKeySize,
		KeySuffixRange: c.stressKeySuffixRange,
		N:              200,
	}
	go s.Stress()
	stressers = append(stressers, s)

	c.Size = size
	c.Agents = agents
	c.Stressers = stressers
	c.Names = names
	c.GRPCURLs = grpcURLs
	c.ClientURLs = clientURLs
	return nil
}

func (c *cluster) WaitHealth() error {
	var err error
	// wait 60s to check cluster health.
	// TODO: set it to a reasonable value. It is set that high because
	// follower may use long time to catch up the leader when reboot under
	// reasonable workload (https://github.com/coreos/etcd/issues/2698)
	for i := 0; i < 60; i++ {
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
		cfg := clientv2.Config{
			Endpoints: []string{u},
		}
		c, err := clientv2.New(cfg)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		kapi := clientv2.NewKeysAPI(c)
		_, err = kapi.Set(ctx, "health", "good", nil)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}
