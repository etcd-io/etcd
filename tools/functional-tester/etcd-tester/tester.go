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
			log.Printf("etcd-tester: [round#%d case#%d] injected failure", i, j)

			log.Printf("etcd-tester: [round#%d case#%d] start recovering failure...", i, j)
			if err := f.Recover(tt.cluster, i); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] recovery error: %v", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] recovered failure", i, j)

			if tt.cluster.v2Only {
				log.Printf("etcd-tester: [round#%d case#%d] succeed!", i, j)
				continue
			}

			log.Printf("etcd-tester: [round#%d case#%d] canceling the stressers...", i, j)
			for _, s := range tt.cluster.Stressers {
				s.Cancel()
			}
			log.Printf("etcd-tester: [round#%d case#%d] canceled stressers", i, j)

			log.Printf("etcd-tester: [round#%d case#%d] checking current revisions...", i, j)
			ok := false
			var currentRevision int64
			for k := 0; k < 5; k++ {
				time.Sleep(time.Second)
				revs, err := tt.cluster.getRevision()
				if err != nil {
					if e := tt.cleanup(i, j); e != nil {
						log.Printf("etcd-tester: [round#%d case#%d.%d] cleanup error: %v", i, j, k, e)
						return
					}
					log.Printf("etcd-tester: [round#%d case#%d.%d] failed to get current revisions (%v)", i, j, k, err)
					continue
				}
				if currentRevision, ok = getSameValue(revs); ok {
					break
				} else {
					log.Printf("etcd-tester: [round#%d case#%d.%d] inconsistent current revisions %+v", i, j, k, revs)
				}
			}
			if !ok {
				log.Printf("etcd-tester: [round#%d case#%d] checking current revisions failure...", i, j)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] all members are consistent with current revisions", i, j)

			log.Printf("etcd-tester: [round#%d case#%d] checking current storage hashes...", i, j)
			hashes, err := tt.cluster.getKVHash()
			if err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] getKVHash error (%v)", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
			}
			if _, ok = getSameValue(hashes); !ok {
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] all members are consistent with storage hashes", i, j)

			revToCompact := max(0, currentRevision-10000)
			log.Printf("etcd-tester: [round#%d case#%d] compacting storage at %d (current revision %d)", i, j, revToCompact, currentRevision)
			if err := tt.cluster.compactKV(revToCompact); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] compactKV error (%v)", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
			}
			log.Printf("etcd-tester: [round#%d case#%d] compacted storage", i, j)

			log.Printf("etcd-tester: [round#%d case#%d] restarting the stressers...", i, j)
			for _, s := range tt.cluster.Stressers {
				go s.Stress()
			}

			log.Printf("etcd-tester: [round#%d case#%d] succeed!", i, j)
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

func (c *cluster) getRevision() (map[string]int64, error) {
	revs := make(map[string]int64)
	for _, u := range c.GRPCURLs {
		conn, err := grpc.Dial(u, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
		if err != nil {
			return nil, err
		}
		kvc := pb.NewKVClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := kvc.Range(ctx, &pb.RangeRequest{Key: []byte("foo")})
		if err != nil {
			return nil, err
		}
		cancel()
		revs[u] = resp.Header.Revision
	}
	return revs, nil
}

func (c *cluster) getKVHash() (map[string]int64, error) {
	hashes := make(map[string]int64)
	for _, u := range c.GRPCURLs {
		conn, err := grpc.Dial(u, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
		if err != nil {
			return nil, err
		}
		kvc := pb.NewKVClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := kvc.Hash(ctx, &pb.HashRequest{})
		if resp != nil && err != nil {
			return nil, err
		}
		cancel()
		hashes[u] = int64(resp.Hash)
	}
	return hashes, nil
}

func getSameValue(hashes map[string]int64) (int64, bool) {
	var rv int64
	ok := true
	for _, v := range hashes {
		if rv == 0 {
			rv = v
		}
		if rv != v {
			ok = false
			break
		}
	}
	return rv, ok
}

func max(n1, n2 int64) int64 {
	if n1 > n2 {
		return n1
	}
	return n2
}

func (c *cluster) compactKV(rev int64) error {
	var (
		conn *grpc.ClientConn
		err  error
	)
	for _, u := range c.GRPCURLs {
		conn, err = grpc.Dial(u, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
		if err != nil {
			continue
		}
		kvc := pb.NewKVClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err = kvc.Compact(ctx, &pb.CompactionRequest{Revision: rev})
		cancel()
		if err == nil {
			return nil
		}
	}
	return err
}
