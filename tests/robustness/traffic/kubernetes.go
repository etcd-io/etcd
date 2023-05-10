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
		traffic: kubernetesTraffic{
			averageKeyCount: 5,
			resource:        "pods",
			namespace:       "default",
			writeChoices: []choiceWeight[KubernetesRequestType]{
				{choice: KubernetesUpdate, weight: 75},
				{choice: KubernetesDelete, weight: 15},
				{choice: KubernetesCreate, weight: 10},
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

type KubernetesRequestType string

const (
	KubernetesUpdate KubernetesRequestType = "update"
	KubernetesCreate KubernetesRequestType = "create"
	KubernetesDelete KubernetesRequestType = "delete"
)

func (t kubernetesTraffic) Run(ctx context.Context, clientId int, c *RecordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage, finish <-chan struct{}) {
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
			resp, err := t.Range(ctx, c, keyPrefix, true)
			if err != nil {
				continue
			}
			s.Reset(resp)
			limiter.Wait(ctx)
			watchCtx, cancel := context.WithTimeout(ctx, WatchTimeout)
			for e := range c.Watch(watchCtx, keyPrefix, resp.Header.Revision, true) {
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
				resp, err := t.Range(ctx, c, keyPrefix, true)
				if err != nil {
					continue
				}
				s.Reset(resp)
				limiter.Wait(ctx)
			}
			err := t.Write(ctx, c, ids, s)
			lastWriteFailed = err != nil
			if err != nil {
				continue
			}
			limiter.Wait(ctx)
		}
	})
	g.Wait()
}

func (t kubernetesTraffic) Write(ctx context.Context, c *RecordingClient, ids identity.Provider, s *storage) (err error) {
	writeCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	count := s.Count()
	if count < t.averageKeyCount/2 {
		err = t.Create(writeCtx, c, t.generateKey(), fmt.Sprintf("%d", ids.NewRequestId()))
	} else {
		key, rev := s.PickRandom()
		if rev == 0 {
			return errors.New("storage empty")
		}
		if count > t.averageKeyCount*3/2 {
			err = t.Delete(writeCtx, c, key, rev)
		} else {
			op := pickRandom(t.writeChoices)
			switch op {
			case KubernetesDelete:
				err = t.Delete(writeCtx, c, key, rev)
			case KubernetesUpdate:
				err = t.Update(writeCtx, c, key, fmt.Sprintf("%d", ids.NewRequestId()), rev)
			case KubernetesCreate:
				err = t.Create(writeCtx, c, t.generateKey(), fmt.Sprintf("%d", ids.NewRequestId()))
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

func (t kubernetesTraffic) Range(ctx context.Context, c *RecordingClient, key string, withPrefix bool) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	resp, err := c.Range(ctx, key, withPrefix)
	cancel()
	return resp, err
}

func (t kubernetesTraffic) Create(ctx context.Context, c *RecordingClient, key, value string) error {
	return t.Update(ctx, c, key, value, 0)
}

func (t kubernetesTraffic) Update(ctx context.Context, c *RecordingClient, key, value string, expectedRevision int64) error {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	err := c.CompareRevisionAndPut(ctx, key, value, expectedRevision)
	cancel()
	return err
}

func (t kubernetesTraffic) Delete(ctx context.Context, c *RecordingClient, key string, expectedRevision int64) error {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	err := c.CompareRevisionAndDelete(ctx, key, expectedRevision)
	cancel()
	return err
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
