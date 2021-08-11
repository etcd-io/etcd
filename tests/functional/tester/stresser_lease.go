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
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/functional/rpcpb"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

const (
	// time to live for lease
	defaultTTL      = 120
	defaultTTLShort = 2
)

type leaseStresser struct {
	stype rpcpb.StresserType
	lg    *zap.Logger

	m      *rpcpb.Member
	cli    *clientv3.Client
	ctx    context.Context
	cancel func()

	rateLimiter *rate.Limiter
	// atomicModifiedKey records the number of keys created and deleted during a test case
	atomicModifiedKey        int64
	numLeases                int
	keysPerLease             int
	aliveLeases              *atomicLeases
	alivedLeasesWithShortTTL *atomicLeases
	revokedLeases            *atomicLeases
	shortLivedLeases         *atomicLeases

	runWg   sync.WaitGroup
	aliveWg sync.WaitGroup
}

type atomicLeases struct {
	// rwLock is used to protect read/write access of leases map
	// which are accessed and modified by different goroutines.
	rwLock sync.RWMutex
	leases map[int64]time.Time
}

func (al *atomicLeases) add(leaseID int64, t time.Time) {
	al.rwLock.Lock()
	al.leases[leaseID] = t
	al.rwLock.Unlock()
}

func (al *atomicLeases) update(leaseID int64, t time.Time) {
	al.rwLock.Lock()
	_, ok := al.leases[leaseID]
	if ok {
		al.leases[leaseID] = t
	}
	al.rwLock.Unlock()
}

func (al *atomicLeases) read(leaseID int64) (rv time.Time, ok bool) {
	al.rwLock.RLock()
	rv, ok = al.leases[leaseID]
	al.rwLock.RUnlock()
	return rv, ok
}

func (al *atomicLeases) remove(leaseID int64) {
	al.rwLock.Lock()
	delete(al.leases, leaseID)
	al.rwLock.Unlock()
}

func (al *atomicLeases) getLeasesMap() map[int64]time.Time {
	leasesCopy := make(map[int64]time.Time)
	al.rwLock.RLock()
	for k, v := range al.leases {
		leasesCopy[k] = v
	}
	al.rwLock.RUnlock()
	return leasesCopy
}

func (ls *leaseStresser) setupOnce() error {
	if ls.aliveLeases != nil {
		return nil
	}
	if ls.numLeases == 0 {
		panic("expect numLeases to be set")
	}
	if ls.keysPerLease == 0 {
		panic("expect keysPerLease to be set")
	}

	ls.aliveLeases = &atomicLeases{leases: make(map[int64]time.Time)}
	return nil
}

func (ls *leaseStresser) Stress() error {
	ls.lg.Info(
		"stress START",
		zap.String("stress-type", ls.stype.String()),
		zap.String("endpoint", ls.m.EtcdClientEndpoint),
	)

	if err := ls.setupOnce(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ls.ctx = ctx
	ls.cancel = cancel

	cli, err := ls.m.CreateEtcdClient(grpc.WithBackoffMaxDelay(1 * time.Second))
	if err != nil {
		return fmt.Errorf("%v (%s)", err, ls.m.EtcdClientEndpoint)
	}
	ls.cli = cli

	ls.revokedLeases = &atomicLeases{leases: make(map[int64]time.Time)}
	ls.shortLivedLeases = &atomicLeases{leases: make(map[int64]time.Time)}
	ls.alivedLeasesWithShortTTL = &atomicLeases{leases: make(map[int64]time.Time)}

	ls.runWg.Add(1)
	go ls.run()
	return nil
}

func (ls *leaseStresser) run() {
	defer ls.runWg.Done()
	ls.restartKeepAlives()
	for {
		// the number of keys created and deleted is roughly 2x the number of created keys for an iteration.
		// the rateLimiter therefore consumes 2x ls.numLeases*ls.keysPerLease tokens where each token represents a create/delete operation for key.
		err := ls.rateLimiter.WaitN(ls.ctx, 2*ls.numLeases*ls.keysPerLease)
		if err == context.Canceled {
			return
		}

		ls.lg.Debug(
			"stress creating leases",
			zap.String("stress-type", ls.stype.String()),
			zap.String("endpoint", ls.m.EtcdClientEndpoint),
		)
		ls.createLeases()
		ls.lg.Debug(
			"stress created leases",
			zap.String("stress-type", ls.stype.String()),
			zap.String("endpoint", ls.m.EtcdClientEndpoint),
		)

		ls.lg.Debug(
			"stress dropped leases",
			zap.String("stress-type", ls.stype.String()),
			zap.String("endpoint", ls.m.EtcdClientEndpoint),
		)
		ls.randomlyDropLeases()
		ls.lg.Debug(
			"stress dropped leases",
			zap.String("stress-type", ls.stype.String()),
			zap.String("endpoint", ls.m.EtcdClientEndpoint),
		)
	}
}

func (ls *leaseStresser) restartKeepAlives() {
	for leaseID := range ls.aliveLeases.getLeasesMap() {
		ls.aliveWg.Add(1)
		go func(id int64) {
			ls.keepLeaseAlive(id)
		}(leaseID)
	}
	for leaseID := range ls.alivedLeasesWithShortTTL.getLeasesMap() {
		ls.aliveWg.Add(1)
		go func(id int64) {
			ls.keepLeaseAlive(id)
		}(leaseID)
	}
}

func (ls *leaseStresser) createLeases() {
	ls.createAliveLeasesWithShortTTL()
	ls.createAliveLeases()
	ls.createShortLivedLeases()
}

func (ls *leaseStresser) createAliveLeases() {
	neededLeases := ls.numLeases - len(ls.aliveLeases.getLeasesMap())
	var wg sync.WaitGroup
	for i := 0; i < neededLeases; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			leaseID, err := ls.createLeaseWithKeys(defaultTTL)
			if err != nil {
				ls.lg.Debug(
					"createLeaseWithKeys failed",
					zap.String("endpoint", ls.m.EtcdClientEndpoint),
					zap.Error(err),
				)
				return
			}
			ls.aliveLeases.add(leaseID, time.Now())
			// keep track of all the keep lease alive goroutines
			ls.aliveWg.Add(1)
			go ls.keepLeaseAlive(leaseID)
		}()
	}
	wg.Wait()
}

func (ls *leaseStresser) createAliveLeasesWithShortTTL() {
	neededLeases := 2
	var wg sync.WaitGroup
	for i := 0; i < neededLeases; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			leaseID, err := ls.createLeaseWithKeys(defaultTTLShort)
			if err != nil {
				ls.lg.Debug(
					"createLeaseWithKeys failed",
					zap.String("endpoint", ls.m.EtcdClientEndpoint),
					zap.Error(err),
				)
				return
			}
			ls.lg.Debug("createAliveLeasesWithShortTTL", zap.Int64("lease-id", leaseID))
			ls.alivedLeasesWithShortTTL.add(leaseID, time.Now())
			// keep track of all the keep lease alive goroutines
			ls.aliveWg.Add(1)
			go ls.keepLeaseAlive(leaseID)
		}()
	}
	wg.Wait()
}

func (ls *leaseStresser) createShortLivedLeases() {
	// one round of createLeases() might not create all the short lived leases we want due to failures.
	// thus, we want to create remaining short lived leases in the future round.
	neededLeases := ls.numLeases - len(ls.shortLivedLeases.getLeasesMap())
	var wg sync.WaitGroup
	for i := 0; i < neededLeases; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			leaseID, err := ls.createLeaseWithKeys(defaultTTLShort)
			if err != nil {
				return
			}
			ls.shortLivedLeases.add(leaseID, time.Now())
		}()
	}
	wg.Wait()
}

func (ls *leaseStresser) createLeaseWithKeys(ttl int64) (int64, error) {
	leaseID, err := ls.createLease(ttl)
	if err != nil {
		ls.lg.Debug(
			"createLease failed",
			zap.String("stress-type", ls.stype.String()),
			zap.String("endpoint", ls.m.EtcdClientEndpoint),
			zap.Error(err),
		)
		return -1, err
	}

	ls.lg.Debug(
		"createLease created lease",
		zap.String("stress-type", ls.stype.String()),
		zap.String("endpoint", ls.m.EtcdClientEndpoint),
		zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
	)
	if err := ls.attachKeysWithLease(leaseID); err != nil {
		return -1, err
	}
	return leaseID, nil
}

func (ls *leaseStresser) randomlyDropLeases() {
	var wg sync.WaitGroup
	for l := range ls.aliveLeases.getLeasesMap() {
		wg.Add(1)
		go func(leaseID int64) {
			defer wg.Done()
			dropped, err := ls.randomlyDropLease(leaseID)
			// if randomlyDropLease encountered an error such as context is cancelled, remove the lease from aliveLeases
			// because we can't tell whether the lease is dropped or not.
			if err != nil {
				ls.lg.Debug(
					"randomlyDropLease failed",
					zap.String("endpoint", ls.m.EtcdClientEndpoint),
					zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
					zap.Error(err),
				)
				ls.aliveLeases.remove(leaseID)
				return
			}
			if !dropped {
				return
			}
			ls.lg.Debug(
				"randomlyDropLease dropped a lease",
				zap.String("stress-type", ls.stype.String()),
				zap.String("endpoint", ls.m.EtcdClientEndpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
			)
			ls.revokedLeases.add(leaseID, time.Now())
			ls.aliveLeases.remove(leaseID)
		}(l)
	}
	wg.Wait()
}

func (ls *leaseStresser) createLease(ttl int64) (int64, error) {
	resp, err := ls.cli.Grant(ls.ctx, ttl)
	if err != nil {
		return -1, err
	}
	return int64(resp.ID), nil
}

func (ls *leaseStresser) keepLeaseAlive(leaseID int64) {
	defer ls.aliveWg.Done()
	ctx, cancel := context.WithCancel(ls.ctx)
	stream, err := ls.cli.KeepAlive(ctx, clientv3.LeaseID(leaseID))
	defer func() { cancel() }()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ls.ctx.Done():
			ls.lg.Debug(
				"keepLeaseAlive context canceled",
				zap.String("stress-type", ls.stype.String()),
				zap.String("endpoint", ls.m.EtcdClientEndpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(ls.ctx.Err()),
			)
			// it is  possible that lease expires at invariant checking phase but not at keepLeaseAlive() phase.
			// this scenario is possible when alive lease is just about to expire when keepLeaseAlive() exists and expires at invariant checking phase.
			// to circumvent that scenario, we check each lease before keepalive loop exist to see if it has been renewed in last TTL/2 duration.
			// if it is renewed, this means that invariant checking have at least ttl/2 time before lease expires which is long enough for the checking to finish.
			// if it is not renewed, we remove the lease from the alive map so that the lease doesn't expire during invariant checking
			renewTime, ok := ls.aliveLeases.read(leaseID)
			if ok && renewTime.Add(defaultTTL/2*time.Second).Before(time.Now()) {
				ls.aliveLeases.remove(leaseID)
				ls.lg.Debug(
					"keepLeaseAlive lease has not been renewed, dropped it",
					zap.String("stress-type", ls.stype.String()),
					zap.String("endpoint", ls.m.EtcdClientEndpoint),
					zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				)
			}
			return
		}

		if err != nil {
			ls.lg.Debug(
				"keepLeaseAlive lease creates stream error",
				zap.String("stress-type", ls.stype.String()),
				zap.String("endpoint", ls.m.EtcdClientEndpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(err),
			)
			cancel()
			ctx, cancel = context.WithCancel(ls.ctx)
			stream, err = ls.cli.KeepAlive(ctx, clientv3.LeaseID(leaseID))
			cancel()
			continue
		}
		if err != nil {
			ls.lg.Debug(
				"keepLeaseAlive failed to receive lease keepalive response",
				zap.String("stress-type", ls.stype.String()),
				zap.String("endpoint", ls.m.EtcdClientEndpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(err),
			)
			continue
		}

		ls.lg.Debug(
			"keepLeaseAlive waiting on lease stream",
			zap.String("stress-type", ls.stype.String()),
			zap.String("endpoint", ls.m.EtcdClientEndpoint),
			zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
		)
		leaseRenewTime := time.Now()
		respRC := <-stream
		if respRC == nil {
			ls.lg.Debug(
				"keepLeaseAlive received nil lease keepalive response",
				zap.String("stress-type", ls.stype.String()),
				zap.String("endpoint", ls.m.EtcdClientEndpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
			)
			continue
		}

		// lease expires after TTL become 0
		// don't send keepalive if the lease has expired
		if respRC.TTL <= 0 {
			ls.lg.Debug(
				"keepLeaseAlive stream received lease keepalive response TTL <= 0",
				zap.String("stress-type", ls.stype.String()),
				zap.String("endpoint", ls.m.EtcdClientEndpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Int64("ttl", respRC.TTL),
			)
			ls.aliveLeases.remove(leaseID)
			return
		}
		// renew lease timestamp only if lease is present
		ls.lg.Debug(
			"keepLeaseAlive renewed a lease",
			zap.String("stress-type", ls.stype.String()),
			zap.String("endpoint", ls.m.EtcdClientEndpoint),
			zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
		)
		ls.aliveLeases.update(leaseID, leaseRenewTime)
	}
}

// attachKeysWithLease function attaches keys to the lease.
// the format of key is the concat of leaseID + '_' + '<order of key creation>'
// e.g 5186835655248304152_0 for first created key and 5186835655248304152_1 for second created key
func (ls *leaseStresser) attachKeysWithLease(leaseID int64) error {
	var txnPuts []clientv3.Op
	for j := 0; j < ls.keysPerLease; j++ {
		txnput := clientv3.OpPut(
			fmt.Sprintf("%d%s%d", leaseID, "_", j),
			fmt.Sprintf("bar"),
			clientv3.WithLease(clientv3.LeaseID(leaseID)),
		)
		txnPuts = append(txnPuts, txnput)
	}
	// keep retrying until lease is not found or ctx is being canceled
	for ls.ctx.Err() == nil {
		_, err := ls.cli.Txn(ls.ctx).Then(txnPuts...).Commit()
		if err == nil {
			// since all created keys will be deleted too, the number of operations on keys will be roughly 2x the number of created keys
			atomic.AddInt64(&ls.atomicModifiedKey, 2*int64(ls.keysPerLease))
			return nil
		}
		if rpctypes.Error(err) == rpctypes.ErrLeaseNotFound {
			return err
		}
	}
	return ls.ctx.Err()
}

// randomlyDropLease drops the lease only when the rand.Int(2) returns 1.
// This creates a 50/50 percents chance of dropping a lease
func (ls *leaseStresser) randomlyDropLease(leaseID int64) (bool, error) {
	if rand.Intn(2) != 0 {
		return false, nil
	}

	// keep retrying until a lease is dropped or ctx is being canceled
	for ls.ctx.Err() == nil {
		_, err := ls.cli.Revoke(ls.ctx, clientv3.LeaseID(leaseID))
		if err == nil || rpctypes.Error(err) == rpctypes.ErrLeaseNotFound {
			return true, nil
		}
	}

	ls.lg.Debug(
		"randomlyDropLease error",
		zap.String("stress-type", ls.stype.String()),
		zap.String("endpoint", ls.m.EtcdClientEndpoint),
		zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
		zap.Error(ls.ctx.Err()),
	)
	return false, ls.ctx.Err()
}

func (ls *leaseStresser) Pause() map[string]int {
	return ls.Close()
}

func (ls *leaseStresser) Close() map[string]int {
	ls.cancel()
	ls.runWg.Wait()
	ls.aliveWg.Wait()
	ls.cli.Close()
	ls.lg.Info(
		"stress STOP",
		zap.String("stress-type", ls.stype.String()),
		zap.String("endpoint", ls.m.EtcdClientEndpoint),
	)
	return nil
}

func (ls *leaseStresser) ModifiedKeys() int64 {
	return atomic.LoadInt64(&ls.atomicModifiedKey)
}
