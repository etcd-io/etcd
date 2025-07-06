// Copyright 2018 The etcd Authors
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
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/functional/rpcpb"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type keyStresser struct {
	lg *zap.Logger

	m *rpcpb.Member

	weightKVWriteSmall     float64
	weightKVWriteLarge     float64
	weightKVReadOneKey     float64
	weightKVReadRange      float64
	weightKVDeleteOneKey   float64
	weightKVDeleteRange    float64
	weightKVTxnWriteDelete float64

	keySize           int
	keyLargeSize      int
	keySuffixRange    int
	keyTxnSuffixRange int
	keyTxnOps         int

	rateLimiter *rate.Limiter

	wg       sync.WaitGroup
	clientsN int

	ctx    context.Context
	cancel func()
	cli    *clientv3.Client

	emu    sync.RWMutex
	ems    map[string]int
	paused bool

	// atomicModifiedKeys records the number of keys created and deleted by the stresser.
	atomicModifiedKeys int64

	stressTable *stressTable
}

func (s *keyStresser) Stress() error {
	var err error
	s.cli, err = s.m.CreateEtcdClient(grpc.WithBackoffMaxDelay(1 * time.Second))
	if err != nil {
		return fmt.Errorf("%v (%q)", err, s.m.EtcdClientEndpoint)
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.wg.Add(s.clientsN)

	s.stressTable = createStressTable([]stressEntry{
		{weight: s.weightKVWriteSmall, f: newStressPut(s.cli, s.keySuffixRange, s.keySize)},
		{weight: s.weightKVWriteLarge, f: newStressPut(s.cli, s.keySuffixRange, s.keyLargeSize)},
		{weight: s.weightKVReadOneKey, f: newStressRange(s.cli, s.keySuffixRange)},
		{weight: s.weightKVReadRange, f: newStressRangeInterval(s.cli, s.keySuffixRange)},
		{weight: s.weightKVDeleteOneKey, f: newStressDelete(s.cli, s.keySuffixRange)},
		{weight: s.weightKVDeleteRange, f: newStressDeleteInterval(s.cli, s.keySuffixRange)},
		{weight: s.weightKVTxnWriteDelete, f: newStressTxn(s.cli, s.keyTxnSuffixRange, s.keyTxnOps)},
	})

	s.emu.Lock()
	s.paused = false
	s.ems = make(map[string]int, 100)
	s.emu.Unlock()
	for i := 0; i < s.clientsN; i++ {
		go s.run()
	}

	s.lg.Info(
		"stress START",
		zap.String("stress-type", "KV"),
		zap.String("endpoint", s.m.EtcdClientEndpoint),
	)
	return nil
}

func (s *keyStresser) run() {
	defer s.wg.Done()

	for {
		if err := s.rateLimiter.Wait(s.ctx); err == context.Canceled {
			return
		}

		// TODO: 10-second is enough timeout to cover leader failure
		// and immediate leader election. Find out what other cases this
		// could be timed out.
		sctx, scancel := context.WithTimeout(s.ctx, 10*time.Second)
		modifiedKeys, err := s.stressTable.choose()(sctx)
		scancel()
		if err == nil {
			atomic.AddInt64(&s.atomicModifiedKeys, modifiedKeys)
			continue
		}

		if !s.isRetryableError(err) {
			return
		}

		// only record errors before pausing stressers
		s.emu.Lock()
		if !s.paused {
			s.ems[err.Error()]++
		}
		s.emu.Unlock()
	}
}

func (s *keyStresser) isRetryableError(err error) bool {
	switch rpctypes.ErrorDesc(err) {
	// retryable
	case context.DeadlineExceeded.Error():
		// This retries when request is triggered at the same time as
		// leader failure. When we terminate the leader, the request to
		// that leader cannot be processed, and times out. Also requests
		// to followers cannot be forwarded to the old leader, so timing out
		// as well. We want to keep stressing until the cluster elects a
		// new leader and start processing requests again.
		return true
	case etcdserver.ErrTimeoutDueToLeaderFail.Error(), etcdserver.ErrTimeout.Error():
		// This retries when request is triggered at the same time as
		// leader failure and follower nodes receive time out errors
		// from losing their leader. Followers should retry to connect
		// to the new leader.
		return true
	case etcdserver.ErrStopped.Error():
		// one of the etcd nodes stopped from failure injection
		return true
	case rpctypes.ErrNotCapable.Error():
		// capability check has not been done (in the beginning)
		return true
	case rpctypes.ErrTooManyRequests.Error():
		// hitting the recovering member.
		return true
	case raft.ErrProposalDropped.Error():
		// removed member, or leadership has changed (old leader got raftpb.MsgProp)
		return true

	// not retryable.
	case context.Canceled.Error():
		// from stresser.Cancel method:
		return false
	}

	if status.Convert(err).Code() == codes.Unavailable {
		// gRPC connection errors are translated to status.Unavailable
		return true
	}

	s.lg.Warn(
		"stress run exiting",
		zap.String("stress-type", "KV"),
		zap.String("endpoint", s.m.EtcdClientEndpoint),
		zap.String("error-type", reflect.TypeOf(err).String()),
		zap.String("error-desc", rpctypes.ErrorDesc(err)),
		zap.Error(err),
	)
	return false
}

func (s *keyStresser) Pause() map[string]int {
	return s.Close()
}

func (s *keyStresser) Close() map[string]int {
	s.cancel()
	s.cli.Close()
	s.wg.Wait()

	s.emu.Lock()
	s.paused = true
	ess := s.ems
	s.ems = make(map[string]int, 100)
	s.emu.Unlock()

	s.lg.Info(
		"stress STOP",
		zap.String("stress-type", "KV"),
		zap.String("endpoint", s.m.EtcdClientEndpoint),
	)
	return ess
}

func (s *keyStresser) ModifiedKeys() int64 {
	return atomic.LoadInt64(&s.atomicModifiedKeys)
}

type stressFunc func(ctx context.Context) (modifiedKeys int64, err error)

type stressEntry struct {
	weight float64
	f      stressFunc
}

type stressTable struct {
	entries    []stressEntry
	sumWeights float64
}

func createStressTable(entries []stressEntry) *stressTable {
	st := stressTable{entries: entries}
	for _, entry := range st.entries {
		st.sumWeights += entry.weight
	}
	return &st
}

func (st *stressTable) choose() stressFunc {
	v := rand.Float64() * st.sumWeights
	var sum float64
	var idx int
	for i := range st.entries {
		sum += st.entries[i].weight
		if sum >= v {
			idx = i
			break
		}
	}
	return st.entries[idx].f
}

func newStressPut(cli *clientv3.Client, keySuffixRange, keySize int) stressFunc {
	return func(ctx context.Context) (int64, error) {
		_, err := cli.Put(
			ctx,
			fmt.Sprintf("foo%016x", rand.Intn(keySuffixRange)),
			string(randBytes(keySize)),
		)
		return 1, err
	}
}

func newStressTxn(cli *clientv3.Client, keyTxnSuffixRange, txnOps int) stressFunc {
	keys := make([]string, keyTxnSuffixRange)
	for i := range keys {
		keys[i] = fmt.Sprintf("/k%03d", i)
	}
	return writeTxn(cli, keys, txnOps)
}

func writeTxn(cli *clientv3.Client, keys []string, txnOps int) stressFunc {
	return func(ctx context.Context) (int64, error) {
		ks := make(map[string]struct{}, txnOps)
		for len(ks) != txnOps {
			ks[keys[rand.Intn(len(keys))]] = struct{}{}
		}
		selected := make([]string, 0, txnOps)
		for k := range ks {
			selected = append(selected, k)
		}
		com, delOp, putOp := getTxnOps(selected[0], "bar00")
		thenOps := []clientv3.Op{delOp}
		elseOps := []clientv3.Op{putOp}
		for i := 1; i < txnOps; i++ { // nested txns
			k, v := selected[i], fmt.Sprintf("bar%02d", i)
			com, delOp, putOp = getTxnOps(k, v)
			txnOp := clientv3.OpTxn(
				[]clientv3.Cmp{com},
				[]clientv3.Op{delOp},
				[]clientv3.Op{putOp},
			)
			thenOps = append(thenOps, txnOp)
			elseOps = append(elseOps, txnOp)
		}
		_, err := cli.Txn(ctx).
			If(com).
			Then(thenOps...).
			Else(elseOps...).
			Commit()
		return int64(txnOps), err
	}
}

func getTxnOps(k, v string) (
	cmp clientv3.Cmp,
	dop clientv3.Op,
	pop clientv3.Op) {
	// if key exists (version > 0)
	cmp = clientv3.Compare(clientv3.Version(k), ">", 0)
	dop = clientv3.OpDelete(k)
	pop = clientv3.OpPut(k, v)
	return cmp, dop, pop
}

func newStressRange(cli *clientv3.Client, keySuffixRange int) stressFunc {
	return func(ctx context.Context) (int64, error) {
		_, err := cli.Get(ctx, fmt.Sprintf("foo%016x", rand.Intn(keySuffixRange)))
		return 0, err
	}
}

func newStressRangeInterval(cli *clientv3.Client, keySuffixRange int) stressFunc {
	return func(ctx context.Context) (int64, error) {
		start := rand.Intn(keySuffixRange)
		end := start + 500
		_, err := cli.Get(
			ctx,
			fmt.Sprintf("foo%016x", start),
			clientv3.WithRange(fmt.Sprintf("foo%016x", end)),
		)
		return 0, err
	}
}

func newStressDelete(cli *clientv3.Client, keySuffixRange int) stressFunc {
	return func(ctx context.Context) (int64, error) {
		_, err := cli.Delete(ctx, fmt.Sprintf("foo%016x", rand.Intn(keySuffixRange)))
		return 1, err
	}
}

func newStressDeleteInterval(cli *clientv3.Client, keySuffixRange int) stressFunc {
	return func(ctx context.Context) (int64, error) {
		start := rand.Intn(keySuffixRange)
		end := start + 500
		resp, err := cli.Delete(ctx,
			fmt.Sprintf("foo%016x", start),
			clientv3.WithRange(fmt.Sprintf("foo%016x", end)),
		)
		if err == nil {
			return resp.Deleted, nil
		}
		return 0, err
	}
}
