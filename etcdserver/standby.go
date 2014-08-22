/*
Copyright 2014 CoreOS Inc.

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

package etcdserver

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/etcd/conf"
)

var (
	noneId int64 = -1
)

type standby struct {
	client  *v2client
	peerHub *peerHub

	leader      int64
	leaderAddr  string
	mu          sync.RWMutex
	clusterConf *conf.ClusterConfig

	*http.ServeMux
}

func newStandby(client *v2client, peerHub *peerHub) *standby {
	s := &standby{
		client:  client,
		peerHub: peerHub,

		leader:      noneId,
		leaderAddr:  "",
		clusterConf: conf.NewClusterConfig(),

		ServeMux: http.NewServeMux(),
	}
	s.Handle("/", handlerErr(s.serveRedirect))
	return s
}

func (s *standby) run(stop chan struct{}) {
	syncDuration := time.Millisecond * 100
	nodes := s.peerHub.getSeeds()
	for {
		select {
		case <-time.After(syncDuration):
		case <-stop:
			log.Printf("standby.stop\n")
			return
		}

		if update, err := s.syncCluster(nodes); err != nil {
			log.Printf("standby.run syncErr=\"%v\"", err)
			continue
		} else {
			nodes = update
		}
		syncDuration = time.Duration(s.clusterConf.SyncInterval * float64(time.Second))
		if s.clusterConf.ActiveSize <= len(nodes) {
			continue
		}
		log.Printf("standby.end\n")
		return
	}
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

func (s *standby) syncCluster(nodes map[string]bool) (map[string]bool, error) {
	for node := range nodes {
		machines, err := s.client.GetMachines(node)
		if err != nil {
			continue
		}
		cfg, err := s.client.GetClusterConfig(node)
		if err != nil {
			continue
		}
		nn := make(map[string]bool)
		for _, machine := range machines {
			nn[machine.PeerURL] = true
			if machine.State == stateLeader {
				id, err := strconv.ParseInt(machine.Name, 0, 64)
				if err != nil {
					return nil, err
				}
				s.setLeaderInfo(id, machine.PeerURL)
			}
		}
		s.clusterConf = cfg
		return nn, nil
	}
	return nil, fmt.Errorf("unreachable cluster")
}
