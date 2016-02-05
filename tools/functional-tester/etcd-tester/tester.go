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
	"encoding/json"
	"fmt"
	"io"
	"os"
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

type logObject struct {
	Time        string      `json:"time"`
	EventType   string      `json:"event_type"`
	EventCaller string      `json:"event_caller"`
	Round       int         `json:"round"`
	Case        int         `json:"case"`
	Failure     string      `json:"failure_description"`
	Message     interface{} `json:"message"`
}

func logf(w io.Writer, eventType, eventCaller string, roundN, caseN int, failure string, msg interface{}) {
	l := logObject{
		Time:        nowPST(),
		EventType:   eventType,
		EventCaller: eventCaller,
		Round:       roundN,
		Case:        caseN,
		Failure:     failure,
		Message:     msg,
	}
	json.NewEncoder(w).Encode(l)
}

var writer = os.Stdout

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
				logf(writer, "error", "WaitHealth", i, j, f.Desc(), err)
				if err := tt.cleanup(i, j); err != nil {
					logf(writer, "error", "cleanup", i, j, f.Desc(), err)
					return
				}
				continue
			}
			logf(writer, "info", "failure.Inject", i, j, f.Desc(), nil)
			if err := f.Inject(tt.cluster, i); err != nil {
				logf(writer, "error", "failure.Inject", i, j, f.Desc(), err)
				if err := tt.cleanup(i, j); err != nil {
					logf(writer, "error", "cleanup", i, j, f.Desc(), err)
					return
				}
				continue
			}
			logf(writer, "info", "failure.Recover", i, j, f.Desc(), nil)
			if err := f.Recover(tt.cluster, i); err != nil {
				logf(writer, "error", "failure.Recover", i, j, f.Desc(), err)
				if err := tt.cleanup(i, j); err != nil {
					logf(writer, "error", "cleanup", i, j, f.Desc(), err)
					return
				}
				continue
			}

			if tt.cluster.v2Only {
				logf(writer, "success", "v2", i, j, f.Desc(), nil)
				continue
			}

			logf(writer, "info", "cluster.Stressers.Cancel", i, j, f.Desc(), nil)
			for _, s := range tt.cluster.Stressers {
				s.Cancel()
			}

			revisions := make(map[string]int64)
			ok := false
			for k := 0; k < 10; k++ {
				time.Sleep(3 * time.Second)
				logf(writer, "info", "cluster.getRevision", i, j, f.Desc(), nil)
				var err error
				revisions, err = tt.cluster.getRevision()
				if err != nil {
					if e := tt.cleanup(i, j); e != nil {
						logf(writer, "error", "cleanup", i, j, f.Desc(), e)
						return
					}
					logf(writer, "error", "cluster.getRevision", i, j, f.Desc(), err)
					continue
				}
				if ok = isSameValueInMap(revisions); ok {
					logf(writer, "success", "isSameValueInMap(cluster.getRevision)", i, j, f.Desc(), fmt.Sprintf("consistent revisions: %+v", revisions))
					break
				}
			}
			if !ok {
				logf(writer, "error", "cluster.getRevision", i, j, f.Desc(), fmt.Sprintf("inconsistent revisions: %+v", revisions))
				if err := tt.cleanup(i, j); err != nil {
					logf(writer, "error", "cleanup", i, j, f.Desc(), err)
					return
				}
				continue
			}

			logf(writer, "info", "cluster.getKVHash", i, j, f.Desc(), nil)
			hashes, err := tt.cluster.getKVHash()
			if err != nil {
				logf(writer, "error", "cluster.getKVHash", i, j, f.Desc(), err)
				if err := tt.cleanup(i, j); err != nil {
					logf(writer, "error", "cleanup", i, j, f.Desc(), err)
					return
				}
			}
			if !isSameValueInMap(hashes) {
				if err := tt.cleanup(i, j); err != nil {
					logf(writer, "error", "cleanup", i, j, f.Desc(), err)
					return
				}
				continue
			}
			logf(writer, "success", "v3", i, j, f.Desc(), "all members are consistent!")

			logf(writer, "info", "stresser.Stress", i, j, f.Desc(), nil)
			for _, s := range tt.cluster.Stressers {
				go s.Stress()
			}
		}
	}
}

func (tt *tester) cleanup(i, j int) error {
	logf(writer, "info", "cleanup", i, j, tt.failures[j].Desc(), nil)
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

func isSameValueInMap(hashes map[string]int64) bool {
	if len(hashes) < 2 {
		return true
	}
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
	return ok
}

func nowPST() string {
	tzone, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.Now().String()[:21]
	}
	return time.Now().In(tzone).String()[:21]
}
