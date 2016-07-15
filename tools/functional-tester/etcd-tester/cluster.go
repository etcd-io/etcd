// Copyright 2015 The etcd Authors
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
	"math/rand"
	"net"
	"strings"
	"time"

	"golang.org/x/net/context"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
	"google.golang.org/grpc"
)

const (
	peerURLPort   = 2380
	failpointPort = 2381
)

type cluster struct {
	v2Only bool // to be deprecated

	datadir              string
	stressQPS            int
	stressKeySize        int
	stressKeySuffixRange int

	Size      int
	Stressers []Stresser

	Members []*member
}

type ClusterStatus struct {
	AgentStatuses map[string]client.Status
}

// newCluster starts and returns a new cluster. The caller should call Terminate when finished, to shut it down.
func newCluster(agentEndpoints []string, datadir string, stressQPS, stressKeySize, stressKeySuffixRange int, isV2Only bool) (*cluster, error) {
	c := &cluster{
		v2Only:               isV2Only,
		datadir:              datadir,
		stressQPS:            stressQPS,
		stressKeySize:        stressKeySize,
		stressKeySuffixRange: stressKeySuffixRange,
	}
	if err := c.bootstrap(agentEndpoints); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *cluster) bootstrap(agentEndpoints []string) error {
	size := len(agentEndpoints)

	members := make([]*member, size)
	memberNameURLs := make([]string, size)
	for i, u := range agentEndpoints {
		agent, err := client.NewAgent(u)
		if err != nil {
			return err
		}
		host, _, err := net.SplitHostPort(u)
		if err != nil {
			return err
		}
		members[i] = &member{
			Agent:        agent,
			Endpoint:     u,
			Name:         fmt.Sprintf("etcd-%d", i),
			ClientURL:    fmt.Sprintf("http://%s:2379", host),
			PeerURL:      fmt.Sprintf("http://%s:%d", host, peerURLPort),
			FailpointURL: fmt.Sprintf("http://%s:%d", host, failpointPort),
		}
		memberNameURLs[i] = members[i].ClusterEntry()
	}
	clusterStr := strings.Join(memberNameURLs, ",")
	token := fmt.Sprint(rand.Int())

	for i, m := range members {
		flags := append(
			m.Flags(),
			"--data-dir", c.datadir,
			"--initial-cluster-token", token,
			"--initial-cluster", clusterStr)

		if _, err := m.Agent.Start(flags...); err != nil {
			// cleanup
			for _, m := range members[:i] {
				m.Agent.Terminate()
			}
			return err
		}
	}

	// TODO: Too intensive stressers can panic etcd member with
	// 'out of memory' error. Put rate limits in server side.
	stressN := 100
	c.Stressers = make([]Stresser, len(members))
	for i, m := range members {
		if c.v2Only {
			c.Stressers[i] = &stresserV2{
				Endpoint:       m.ClientURL,
				KeySize:        c.stressKeySize,
				KeySuffixRange: c.stressKeySuffixRange,
				N:              stressN,
			}
		} else {
			c.Stressers[i] = &stresser{
				Endpoint:       m.grpcAddr(),
				KeySize:        c.stressKeySize,
				KeySuffixRange: c.stressKeySuffixRange,
				qps:            c.stressQPS,
				N:              stressN,
			}
		}
		go c.Stressers[i].Stress()
	}

	c.Size = size
	c.Members = members
	return nil
}

func (c *cluster) Reset() error {
	eps := make([]string, len(c.Members))
	for i, m := range c.Members {
		eps[i] = m.Endpoint
	}
	return c.bootstrap(eps)
}

func (c *cluster) WaitHealth() error {
	var err error
	// wait 60s to check cluster health.
	// TODO: set it to a reasonable value. It is set that high because
	// follower may use long time to catch up the leader when reboot under
	// reasonable workload (https://github.com/coreos/etcd/issues/2698)
	healthFunc := func(m *member) error { return m.SetHealthKeyV3() }
	if c.v2Only {
		healthFunc = func(m *member) error { return m.SetHealthKeyV2() }
	}
	for i := 0; i < 60; i++ {
		for _, m := range c.Members {
			if err = healthFunc(m); err != nil {
				break
			}
		}
		if err == nil {
			return nil
		}
		plog.Warningf("#%d setHealthKey error (%v)", i, err)
		time.Sleep(time.Second)
	}
	return err
}

// GetLeader returns the index of leader and error if any.
func (c *cluster) GetLeader() (int, error) {
	if c.v2Only {
		return 0, nil
	}
	for i, m := range c.Members {
		isLeader, err := m.IsLeader()
		if isLeader || err != nil {
			return i, err
		}
	}
	return 0, fmt.Errorf("no leader found")
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
	for _, m := range c.Members {
		if err := m.Agent.Cleanup(); err != nil {
			lasterr = err
		}
	}
	for _, s := range c.Stressers {
		s.Cancel()
	}
	return lasterr
}

func (c *cluster) Terminate() {
	for _, m := range c.Members {
		m.Agent.Terminate()
	}
	for _, s := range c.Stressers {
		s.Cancel()
	}
}

func (c *cluster) Status() ClusterStatus {
	cs := ClusterStatus{
		AgentStatuses: make(map[string]client.Status),
	}

	for _, m := range c.Members {
		s, err := m.Agent.Status()
		// TODO: add a.Desc() as a key of the map
		desc := m.Endpoint
		if err != nil {
			cs.AgentStatuses[desc] = client.Status{State: "unknown"}
			plog.Printf("failed to get the status of agent [%s]", desc)
		}
		cs.AgentStatuses[desc] = s
	}
	return cs
}

func (c *cluster) getRevisionHash() (map[string]int64, map[string]int64, error) {
	revs := make(map[string]int64)
	hashes := make(map[string]int64)
	for _, m := range c.Members {
		rev, hash, err := m.RevHash()
		if err != nil {
			return nil, nil, err
		}
		revs[m.ClientURL] = rev
		hashes[m.ClientURL] = hash
	}
	return revs, hashes, nil
}

func (c *cluster) compactKV(rev int64, timeout time.Duration) (err error) {
	if rev <= 0 {
		return nil
	}

	for i, m := range c.Members {
		u := m.ClientURL
		conn, derr := m.dialGRPC()
		if derr != nil {
			plog.Printf("[compact kv #%d] dial error %v (endpoint %s)", i, derr, u)
			err = derr
			continue
		}
		kvc := pb.NewKVClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		plog.Printf("[compact kv #%d] starting (endpoint %s)", i, u)
		_, cerr := kvc.Compact(ctx, &pb.CompactionRequest{Revision: rev, Physical: true}, grpc.FailFast(false))
		cancel()
		conn.Close()
		succeed := true
		if cerr != nil {
			if strings.Contains(cerr.Error(), "required revision has been compacted") && i > 0 {
				plog.Printf("[compact kv #%d] already compacted (endpoint %s)", i, u)
			} else {
				plog.Warningf("[compact kv #%d] error %v (endpoint %s)", i, cerr, u)
				err = cerr
				succeed = false
			}
		}
		if succeed {
			plog.Printf("[compact kv #%d] done (endpoint %s)", i, u)
		}
	}
	return err
}

func (c *cluster) checkCompact(rev int64) error {
	if rev == 0 {
		return nil
	}
	for _, m := range c.Members {
		if err := m.CheckCompact(rev); err != nil {
			return err
		}
	}
	return nil
}

func (c *cluster) defrag() error {
	for _, m := range c.Members {
		if err := m.Defrag(); err != nil {
			return err
		}
	}
	return nil
}
