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

package etcd

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/store"
)

var (
	noneId int64 = -1
)

type standby struct {
	id          int64
	pubAddr     string
	raftPubAddr string

	client  *v2client
	peerHub *peerHub

	nodes map[string]bool

	leader      int64
	leaderAddr  string
	mu          sync.RWMutex
	clusterConf *config.ClusterConfig

	stopc chan struct{}

	*http.ServeMux
}

func newStandby(id int64, pubAddr string, raftPubAddr string, nodes map[string]bool, client *v2client, peerHub *peerHub) *standby {
	s := &standby{
		id:          id,
		pubAddr:     pubAddr,
		raftPubAddr: raftPubAddr,

		client:  client,
		peerHub: peerHub,

		nodes: nodes,

		leader:      noneId,
		leaderAddr:  "",
		clusterConf: config.NewClusterConfig(),

		stopc: make(chan struct{}),

		ServeMux: http.NewServeMux(),
	}
	s.Handle("/", handlerErr(s.serveRedirect))
	return s
}

func (s *standby) run() int64 {
	var syncDuration time.Duration
	for {
		select {
		case <-time.After(syncDuration):
		case <-s.stopc:
			log.Printf("Standby %d stopped\n", s.id)
			return stopMode
		}

		if err := s.syncCluster(); err != nil {
			log.Println("standby sync:", err)
			continue
		}
		syncDuration = time.Duration(s.clusterConf.SyncInterval * float64(time.Second))
		if s.clusterConf.ActiveSize <= len(s.nodes) {
			continue
		}
		if err := s.joinByAddr(s.leaderAddr); err != nil {
			log.Println("standby join:", err)
			continue
		}
		return participantMode
	}
}

func (s *standby) stop() {
	close(s.stopc)
}

func (s *standby) leaderInfo() (int64, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.leader, s.leaderAddr
}

func (s *standby) setLeaderInfo(leader int64, leaderAddr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leader, s.leaderAddr = leader, leaderAddr
}

func (s *standby) serveRedirect(w http.ResponseWriter, r *http.Request) error {
	leader, leaderAddr := s.leaderInfo()
	if leader == noneId {
		return fmt.Errorf("no leader in the cluster")
	}
	redirectAddr, err := buildRedirectURL(leaderAddr, r.URL)
	if err != nil {
		return err
	}
	http.Redirect(w, r, redirectAddr, http.StatusTemporaryRedirect)
	return nil
}

func (s *standby) syncCluster() error {
	for node := range s.nodes {
		machines, err := s.client.GetMachines(node)
		if err != nil {
			continue
		}
		config, err := s.client.GetClusterConfig(node)
		if err != nil {
			continue
		}
		s.nodes = make(map[string]bool)
		for _, machine := range machines {
			s.nodes[machine.PeerURL] = true
			if machine.State == stateLeader {
				id, err := strconv.ParseInt(machine.Name, 0, 64)
				if err != nil {
					return err
				}
				s.setLeaderInfo(id, machine.PeerURL)
			}
		}
		s.clusterConf = config
		return nil
	}
	return fmt.Errorf("unreachable cluster")
}

func (s *standby) joinByAddr(addr string) error {
	info := &context{
		MinVersion: store.MinVersion(),
		MaxVersion: store.MaxVersion(),
		ClientURL:  s.pubAddr,
		PeerURL:    s.raftPubAddr,
	}
	if err := s.client.AddMachine(s.leaderAddr, fmt.Sprint(s.id), info); err != nil {
		return err
	}
	return nil
}
