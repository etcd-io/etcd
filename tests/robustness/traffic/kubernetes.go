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
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/kubernetes"
	"go.etcd.io/etcd/pkg/v3/stringutil"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/random"
)

var (
	Kubernetes Traffic = kubernetesTraffic{
		averageKeyCount: 10,
		resource:        "pods",
		namespace:       "default",
		// Please keep the sum of weights equal 100.
		writeChoices: []random.ChoiceWeight[KubernetesRequestType]{
			{Choice: KubernetesUpdate, Weight: 90},
			{Choice: KubernetesDelete, Weight: 5},
			{Choice: KubernetesCreate, Weight: 5},
		},
	}

	KubernetesCreateDelete Traffic = kubernetesTraffic{
		averageKeyCount: 10,
		resource:        "pods",
		namespace:       "default",
		// Please keep the sum of weights equal 100.
		writeChoices: []random.ChoiceWeight[KubernetesRequestType]{
			{Choice: KubernetesDelete, Weight: 40},
			{Choice: KubernetesCreate, Weight: 60},
		},
	}
)

type kubernetesTraffic struct {
	averageKeyCount int
	resource        string
	namespace       string
	writeChoices    []random.ChoiceWeight[KubernetesRequestType]
}

func (t kubernetesTraffic) ExpectUniqueRevision() bool {
	return true
}

func (t kubernetesTraffic) RunTrafficLoop(ctx context.Context, c *client.RecordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIDStorage, nonUniqueWriteLimiter ConcurrencyLimiter, finish <-chan struct{}) {
	kc := kubernetes.Client{Client: &clientv3.Client{KV: c}}
	s := newStorage()
	keyPrefix := "/registry/" + t.resource + "/"
	g := errgroup.Group{}
	readLimit := t.averageKeyCount

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-finish:
				return nil
			default:
			}
			rev, err := t.Read(ctx, kc, s, limiter, keyPrefix, readLimit)
			if err != nil {
				continue
			}
			t.Watch(ctx, c, s, limiter, keyPrefix, rev+1)
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
				_, err := t.Read(ctx, kc, s, limiter, keyPrefix, 0)
				if err != nil {
					continue
				}
			}
			err := t.Write(ctx, kc, ids, s, limiter, nonUniqueWriteLimiter)
			lastWriteFailed = err != nil
			if err != nil {
				continue
			}
		}
	})
	g.Wait()
}

func (t kubernetesTraffic) Read(ctx context.Context, kc kubernetes.Interface, s *storage, limiter *rate.Limiter, keyPrefix string, limit int) (rev int64, err error) {
	hasMore := true
	var kvs []*mvccpb.KeyValue
	var revision int64
	var cont string

	for hasMore {
		readCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
		resp, err := kc.List(readCtx, keyPrefix, kubernetes.ListOptions{Continue: cont, Revision: revision, Limit: int64(limit)})
		cancel()
		if err != nil {
			return 0, err
		}
		limiter.Wait(ctx)

		kvs = append(kvs, resp.Kvs...)
		if revision == 0 {
			revision = resp.Revision
		}
		hasMore = resp.Count > int64(len(resp.Kvs))
		if hasMore {
			cont = string(kvs[len(kvs)-1].Key) + "\x00"
		}
	}
	s.Reset(revision, kvs)
	return revision, nil
}

func (t kubernetesTraffic) Write(ctx context.Context, kc kubernetes.Interface, ids identity.Provider, s *storage, limiter *rate.Limiter, nonUniqueWriteLimiter ConcurrencyLimiter) (err error) {
	writeCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	count := s.Count()
	if count < t.averageKeyCount/2 {
		_, err = kc.OptimisticPut(writeCtx, t.generateKey(), []byte(fmt.Sprintf("%d", ids.NewRequestID())), 0, kubernetes.PutOptions{})
	} else {
		key, rev := s.PickRandom()
		if rev == 0 {
			return errors.New("storage empty")
		}
		if count > t.averageKeyCount*3/2 && nonUniqueWriteLimiter.Take() {
			_, err = kc.OptimisticDelete(writeCtx, key, rev, kubernetes.DeleteOptions{GetOnFailure: true})
			nonUniqueWriteLimiter.Return()
		} else {
			shouldReturn := false

			choices := t.writeChoices
			if shouldReturn = nonUniqueWriteLimiter.Take(); !shouldReturn {
				choices = filterOutNonUniqueKubernetesWrites(t.writeChoices)
			}
			op := random.PickRandom(choices)
			switch op {
			case KubernetesDelete:
				_, err = kc.OptimisticDelete(writeCtx, key, rev, kubernetes.DeleteOptions{GetOnFailure: true})
			case KubernetesUpdate:
				_, err = kc.OptimisticPut(writeCtx, key, []byte(fmt.Sprintf("%d", ids.NewRequestID())), rev, kubernetes.PutOptions{GetOnFailure: true})
			case KubernetesCreate:
				_, err = kc.OptimisticPut(writeCtx, t.generateKey(), []byte(fmt.Sprintf("%d", ids.NewRequestID())), 0, kubernetes.PutOptions{})
			default:
				panic(fmt.Sprintf("invalid choice: %q", op))
			}
			if shouldReturn {
				nonUniqueWriteLimiter.Return()
			}
		}
	}
	if err != nil {
		return err
	}
	limiter.Wait(ctx)
	return nil
}

func filterOutNonUniqueKubernetesWrites(choices []random.ChoiceWeight[KubernetesRequestType]) (resp []random.ChoiceWeight[KubernetesRequestType]) {
	for _, choice := range choices {
		if choice.Choice != KubernetesDelete {
			resp = append(resp, choice)
		}
	}
	return resp
}

func (t kubernetesTraffic) Watch(ctx context.Context, c *client.RecordingClient, s *storage, limiter *rate.Limiter, keyPrefix string, revision int64) {
	watchCtx, cancel := context.WithTimeout(ctx, WatchTimeout)
	defer cancel()

	// Kubernetes issues Watch requests by requiring a leader to exist
	// in the cluster:
	// https://github.com/kubernetes/kubernetes/blob/2016fab3085562b4132e6d3774b6ded5ba9939fd/staging/src/k8s.io/apiserver/pkg/storage/etcd3/store.go#L872
	watchCtx = clientv3.WithRequireLeader(watchCtx)
	for e := range c.Watch(watchCtx, keyPrefix, revision, true, true, true) {
		s.Update(e)
	}
	limiter.Wait(ctx)
}

func (t kubernetesTraffic) generateKey() string {
	return fmt.Sprintf("/registry/%s/%s/%s", t.resource, t.namespace, stringutil.RandString(5))
}

func (t kubernetesTraffic) RunCompactLoop(ctx context.Context, c *client.RecordingClient, interval time.Duration, finish <-chan struct{}) {
	// Based on https://github.com/kubernetes/apiserver/blob/7dd4904f1896e11244ba3c5a59797697709de6b6/pkg/storage/etcd3/compact.go#L112-L127
	var compactTime int64
	var rev int64
	var err error
	for {
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return
		case <-finish:
			return
		}

		compactTime, rev, err = compact(ctx, c, compactTime, rev)
		if err != nil {
			continue
		}
	}
}

// Based on https://github.com/kubernetes/apiserver/blob/7dd4904f1896e11244ba3c5a59797697709de6b6/pkg/storage/etcd3/compact.go#L30
const (
	compactRevKey = "compact_rev_key"
)

func compact(ctx context.Context, client *client.RecordingClient, t, rev int64) (int64, int64, error) {
	// Based on https://github.com/kubernetes/apiserver/blob/7dd4904f1896e11244ba3c5a59797697709de6b6/pkg/storage/etcd3/compact.go#L133-L162
	resp, err := client.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(compactRevKey), "=", t)).
		Then(clientv3.OpPut(compactRevKey, strconv.FormatInt(rev, 10))).
		Else(clientv3.OpGet(compactRevKey)).
		Commit()
	if err != nil {
		return t, rev, err
	}

	curRev := resp.Header.Revision

	if !resp.Succeeded {
		curTime := resp.Responses[0].GetResponseRange().Kvs[0].Version
		return curTime, curRev, nil
	}
	curTime := t + 1

	if rev == 0 {
		return curTime, curRev, nil
	}
	if _, err = client.Compact(ctx, rev); err != nil {
		return curTime, curRev, err
	}
	return curTime, curRev, nil
}

type KubernetesRequestType string

const (
	KubernetesDelete KubernetesRequestType = "delete"
	KubernetesUpdate KubernetesRequestType = "update"
	KubernetesCreate KubernetesRequestType = "create"
)

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

func (s *storage) Reset(revision int64, kvs []*mvccpb.KeyValue) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if revision <= s.revision {
		return
	}
	s.keyRevision = make(map[string]int64, len(kvs))
	for _, kv := range kvs {
		s.keyRevision[string(kv.Key)] = kv.ModRevision
	}
	s.revision = revision
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
