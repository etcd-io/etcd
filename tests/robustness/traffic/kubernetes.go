// Copyright 2023 The etcd Authors
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

package traffic

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/stringutil"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
)

var (
	KubernetesTraffic = Config{
		Name:        "Kubernetes",
		minimalQPS:  200,
		maximalQPS:  1000,
		clientCount: 12,
		Traffic: kubernetesTraffic{
			averageKeyCount: 10,
			resource:        "pods",
			namespace:       "default",
			writeChoices: []choiceWeight[KubernetesRequestType]{
				{choice: KubernetesUpdate, weight: 90},
				{choice: KubernetesDelete, weight: 5},
				{choice: KubernetesCreate, weight: 5},
			},
		},
	}
)

type kubernetesTraffic struct {
	averageKeyCount int
	resource        string
	namespace       string
	writeChoices    []choiceWeight[KubernetesRequestType]
}

func (t kubernetesTraffic) ExpectUniqueRevision() bool {
	return true
}

func (t kubernetesTraffic) Run(ctx context.Context, c *RecordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage, finish <-chan struct{}) {
	kc := &kubernetesClient{client: c}
	s := newStorage()
	keyPrefix := "/registry/" + t.resource + "/"
	g := errgroup.Group{}

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-finish:
				return nil
			default:
			}
			listCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
			resp, err := kc.List(listCtx, keyPrefix)
			cancel()
			if err != nil {
				continue
			}
			s.Reset(resp)
			limiter.Wait(ctx)
			watchCtx, cancel := context.WithTimeout(ctx, WatchTimeout)
			for e := range c.Watch(watchCtx, keyPrefix, resp.Header.Revision+1, true, true) {
				s.Update(e)
			}
			cancel()
		}
	})
	g.Go(func() error {
		lastWriteFailed := false
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-finish:
				return nil
			default:
			}
			// Avoid multiple failed writes in a row
			if lastWriteFailed {
				listCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
				resp, err := kc.List(listCtx, keyPrefix)
				cancel()
				if err != nil {
					continue
				}
				s.Reset(resp)
				limiter.Wait(ctx)
			}
			writeCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
			err := t.Write(writeCtx, kc, ids, s)
			cancel()
			lastWriteFailed = err != nil
			if err != nil {
				continue
			}
			limiter.Wait(ctx)
		}
	})
	g.Wait()
}

func (t kubernetesTraffic) Write(ctx context.Context, kc *kubernetesClient, ids identity.Provider, s *storage) (err error) {
	count := s.Count()
	if count < t.averageKeyCount/2 {
		err = kc.OptimisticCreate(ctx, t.generateKey(), fmt.Sprintf("%d", ids.NewRequestId()))
	} else {
		key, rev := s.PickRandom()
		if rev == 0 {
			return errors.New("storage empty")
		}
		if count > t.averageKeyCount*3/2 {
			_, err = kc.OptimisticDelete(ctx, key, rev)
		} else {
			op := pickRandom(t.writeChoices)
			switch op {
			case KubernetesDelete:
				_, err = kc.OptimisticDelete(ctx, key, rev)
			case KubernetesUpdate:
				_, err = kc.OptimisticUpdate(ctx, key, fmt.Sprintf("%d", ids.NewRequestId()), rev)
			case KubernetesCreate:
				err = kc.OptimisticCreate(ctx, t.generateKey(), fmt.Sprintf("%d", ids.NewRequestId()))
			default:
				panic(fmt.Sprintf("invalid choice: %q", op))
			}
		}
	}
	return err
}

func (t kubernetesTraffic) generateKey() string {
	return fmt.Sprintf("/registry/%s/%s/%s", t.resource, t.namespace, stringutil.RandString(5))
}

type KubernetesRequestType string

const (
	KubernetesDelete KubernetesRequestType = "delete"
	KubernetesUpdate KubernetesRequestType = "update"
	KubernetesCreate KubernetesRequestType = "create"
)

type kubernetesClient struct {
	client *RecordingClient
}

func (k kubernetesClient) List(ctx context.Context, key string) (*clientv3.GetResponse, error) {
	resp, err := k.client.Range(ctx, key, true, 0)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (k kubernetesClient) OptimisticDelete(ctx context.Context, key string, expectedRevision int64) (*mvccpb.KeyValue, error) {
	return k.optimisticOperationOrGet(ctx, key, clientv3.OpDelete(key), expectedRevision)
}

func (k kubernetesClient) OptimisticUpdate(ctx context.Context, key, value string, expectedRevision int64) (*mvccpb.KeyValue, error) {
	return k.optimisticOperationOrGet(ctx, key, clientv3.OpPut(key, value), expectedRevision)
}

func (k kubernetesClient) OptimisticCreate(ctx context.Context, key, value string) error {
	_, err := k.client.Txn(ctx, []clientv3.Cmp{clientv3.Compare(clientv3.ModRevision(key), "=", 0)}, []clientv3.Op{clientv3.OpPut(key, value)}, nil)
	return err
}

// Kubernetes optimistically assumes that key didn't change since it was last observed, so it executes operations within a transaction conditioned on key not changing.
// However, if the keys value changed it wants imminently to read it, thus the Get operation on failure.
func (k kubernetesClient) optimisticOperationOrGet(ctx context.Context, key string, operation clientv3.Op, expectedRevision int64) (*mvccpb.KeyValue, error) {
	resp, err := k.client.Txn(ctx, []clientv3.Cmp{clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)}, []clientv3.Op{operation}, []clientv3.Op{clientv3.OpGet(key)})
	if err != nil {
		return nil, err
	}
	if !resp.Succeeded {
		getResp := (*clientv3.GetResponse)(resp.Responses[0].GetResponseRange())
		if err != nil || len(getResp.Kvs) == 0 {
			return nil, err
		}
		if len(getResp.Kvs) == 1 {
			return getResp.Kvs[0], err
		}
	}
	return nil, err
}

type storage struct {
	mux         sync.RWMutex
	keyRevision map[string]int64
	revision    int64
}

func newStorage() *storage {
	return &storage{
		keyRevision: map[string]int64{},
	}
}

func (s *storage) Update(resp clientv3.WatchResponse) {
	s.mux.Lock()
	defer s.mux.Unlock()
	for _, e := range resp.Events {
		if e.Kv.ModRevision < s.revision {
			continue
		}
		s.revision = e.Kv.ModRevision
		switch e.Type {
		case mvccpb.PUT:
			s.keyRevision[string(e.Kv.Key)] = e.Kv.ModRevision
		case mvccpb.DELETE:
			delete(s.keyRevision, string(e.Kv.Key))
		}
	}
}

func (s *storage) Reset(resp *clientv3.GetResponse) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if resp.Header.Revision <= s.revision {
		return
	}
	s.keyRevision = make(map[string]int64, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		s.keyRevision[string(kv.Key)] = kv.ModRevision
	}
	s.revision = resp.Header.Revision
}

func (s *storage) Count() int {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return len(s.keyRevision)
}

func (s *storage) PickRandom() (key string, rev int64) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	n := rand.Intn(len(s.keyRevision))
	i := 0
	for k, v := range s.keyRevision {
		if i == n {
			return k, v
		}
		i++
	}
	return "", 0
}
