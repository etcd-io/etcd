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
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	clientV2 "github.com/coreos/etcd/client"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type Stresser interface {
	// Stress starts to stress the etcd cluster
	Stress() error
	// Cancel cancels the stress test on the etcd cluster
	Cancel()
	// Report reports the success and failure of the stress test
	Report() (success int, failure int)
}

type stresser struct {
	Endpoints []string

	KeySize        int
	KeySuffixRange int

	N int

	mu      sync.Mutex
	failure int
	success int

	cancel func()
}

// TODO: use clientv3
func mustCreateConn(endpoint string) *grpc.ClientConn {
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial error: %v\n", err)
		os.Exit(1)
	}
	return conn
}

func (s *stresser) Stress() error {
	endpoint := ""
	for i := range s.Endpoints {
		if s.Endpoints[i] != "" {
			endpoint = s.Endpoints[i]
			break
		}
	}
	if endpoint == "" {
		return fmt.Errorf("no endpoints available")
	}

	connsN := 1
	clientsN := 10
	conns := make([]*grpc.ClientConn, connsN)
	for i := range conns {
		conns[i] = mustCreateConn(endpoint)
	}
	clients := make([]pb.KVClient, clientsN)
	for i := range clients {
		clients[i] = pb.NewKVClient(conns[i%int(connsN)])
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	for i := 0; i < s.N; i++ {
		go func(i int) {
			for {
				kvc := clients[i%len(clients)]
				key := []byte(fmt.Sprintf("foo%d", rand.Intn(s.KeySuffixRange)))
				val := []byte(randStr(s.KeySize))

				putctx, putcancel := context.WithTimeout(ctx, 5*time.Second)
				_, err := kvc.Put(putctx, &pb.PutRequest{
					Key:   key,
					Value: val,
				})
				putcancel()
				if err == context.Canceled {
					return
				}
				s.mu.Lock()
				if err != nil {
					s.failure++
				} else {
					s.success++
				}
				s.mu.Unlock()
			}
		}(i)
	}

	<-ctx.Done()
	return nil
}

func (s *stresser) Cancel() {
	s.cancel()
}

func (s *stresser) Report() (success int, failure int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.success, s.failure
}

type stresserV2 struct {
	Endpoint string

	KeySize        int
	KeySuffixRange int

	N int
	// TODO: not implemented
	Interval time.Duration

	mu      sync.Mutex
	failure int
	success int

	cancel func()
}

func (s *stresserV2) Stress() error {
	cfg := clientV2.Config{
		Endpoints: []string{s.Endpoint},
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			MaxIdleConnsPerHost: s.N,
		},
	}
	c, err := clientV2.New(cfg)
	if err != nil {
		return err
	}

	kv := clientV2.NewKeysAPI(c)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	for i := 0; i < s.N; i++ {
		go func() {
			for {
				setctx, setcancel := context.WithTimeout(ctx, clientV2.DefaultRequestTimeout)
				key := fmt.Sprintf("foo%d", rand.Intn(s.KeySuffixRange))
				_, err := kv.Set(setctx, key, randStr(s.KeySize), nil)
				setcancel()
				if err == context.Canceled {
					return
				}
				s.mu.Lock()
				if err != nil {
					s.failure++
				} else {
					s.success++
				}
				s.mu.Unlock()
			}
		}()
	}

	<-ctx.Done()
	return nil
}

func (s *stresserV2) Cancel() {
	s.cancel()
}

func (s *stresserV2) Report() (success int, failure int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.success, s.failure
}

func randStr(size int) string {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(int('a') + rand.Intn(26))
	}
	return string(data)
}
