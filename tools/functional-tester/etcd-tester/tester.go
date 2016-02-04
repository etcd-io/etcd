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
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type tester struct {
	failures []failure
	cluster  *cluster
	limit    int

	status Status
}

func (tt *tester) runLoop() {
	tt.status.Since = time.Now()
	tt.status.RoundLimit = tt.limit
	tt.status.cluster = tt.cluster
	for _, f := range tt.failures {
		tt.status.Failures = append(tt.status.Failures, f.Desc())
	}
	for i := 0; i < tt.limit; i++ {
		tt.status.setRound(i)

		for j, f := range tt.failures {
			tt.status.setCase(j)

			if err := tt.cluster.WaitHealth(); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] wait full health error: %v", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] start failure %s", i, j, f.Desc())
			log.Printf("etcd-tester: [round#%d case#%d] start injecting failure...", i, j)
			if err := f.Inject(tt.cluster, i); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] injection error: %v", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] start recovering failure...", i, j)
			if err := f.Recover(tt.cluster, i); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] recovery error: %v", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}

			if tt.cluster.v2Only {
				log.Printf("etcd-tester: [round#%d case#%d] succeed!", i, j)
				continue
			}

			log.Printf("etcd-tester: [round#%d case#%d] canceling the stressers...", i, j)
			for _, s := range tt.cluster.Stressers {
				s.Cancel()
			}

			log.Printf("etcd-tester: [round#%d case#%d] waiting 5s for pending PUTs to be committed across cluster...", i, j)
			time.Sleep(5 * time.Second)

			log.Printf("etcd-tester: [round#%d case#%d] starting checking consistency...", i, j)
			err := tt.cluster.checkConsistency()
			if err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] checkConsistency error (%v)", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
			} else {
				log.Printf("etcd-tester: [round#%d case#%d] all members are consistent!", i, j)
				log.Printf("etcd-tester: [round#%d case#%d] succeed!", i, j)
			}

			log.Printf("etcd-tester: [round#%d case#%d] restarting the stressers...", i, j)
			for _, s := range tt.cluster.Stressers {
				go s.Stress()
			}
		}
	}
}

func (tt *tester) cleanup(i, j int) error {
	log.Printf("etcd-tester: [round#%d case#%d] cleaning up...", i, j)
	if err := tt.cluster.Cleanup(); err != nil {
		return err
	}
	return tt.cluster.Bootstrap()
}

type Status struct {
	Since      time.Time
	Failures   []string
	RoundLimit int

	Cluster ClusterStatus
	cluster *cluster

	mu    sync.Mutex // guards Round and Case
	Round int
	Case  int
}

// get gets a copy of status
func (s *Status) get() Status {
	s.mu.Lock()
	got := *s
	cluster := s.cluster
	s.mu.Unlock()
	got.Cluster = cluster.Status()
	return got
}

func (s *Status) setRound(r int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Round = r
}

func (s *Status) setCase(c int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Case = c
}

// checkConsistency stops the cluster for a moment and get the hashes of KV storages.
func (c *cluster) checkConsistency() error {
	hashes := make(map[string]uint32)
	for _, u := range c.GRPCURLs {
		conn, err := grpc.Dial(u, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
		if err != nil {
			return err
		}
		kvc := pb.NewKVClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := kvc.Hash(ctx, &pb.HashRequest{})
		hv := resp.Hash
		if resp != nil && err != nil {
			return err
		}
		cancel()

		hashes[u] = hv
	}

	if !checkConsistency(hashes) {
		return fmt.Errorf("check consistency fails: %v", hashes)
	}
	return nil
}

// checkConsistency returns true if all nodes have the same KV hash values.
func checkConsistency(hashes map[string]uint32) bool {
	var cv uint32
	isConsistent := true
	for _, v := range hashes {
		if cv == 0 {
			cv = v
		}
		if cv != v {
			isConsistent = false
		}
	}
	return isConsistent
}
