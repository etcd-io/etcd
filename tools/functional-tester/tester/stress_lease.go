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

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

const (
	// time to live for lease
	TTL      = 120
	TTLShort = 2
)

type leaseStresser struct {
	logger *zap.Logger

	endpoint string
	cancel   func()
	conn     *grpc.ClientConn
	kvc      pb.KVClient
	lc       pb.LeaseClient
	ctx      context.Context

	rateLimiter *rate.Limiter
	// atomicModifiedKey records the number of keys created and deleted during a test case
	atomicModifiedKey int64
	numLeases         int
	keysPerLease      int

	aliveLeases      *atomicLeases
	revokedLeases    *atomicLeases
	shortLivedLeases *atomicLeases

	runWg   sync.WaitGroup
	aliveWg sync.WaitGroup
}

type atomicLeases struct {
	// rwLock is used to protect read/write access of leases map
	// which are accessed and modified by different go routines.
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
	ls.logger.Info(
		"lease stresser is started",
		zap.String("endpoint", ls.endpoint),
	)

	if err := ls.setupOnce(); err != nil {
		return err
	}

	conn, err := grpc.Dial(ls.endpoint, grpc.WithInsecure(), grpc.WithBackoffMaxDelay(1*time.Second))
	if err != nil {
		return fmt.Errorf("%v (%s)", err, ls.endpoint)
	}
	ls.conn = conn
	ls.kvc = pb.NewKVClient(conn)
	ls.lc = pb.NewLeaseClient(conn)
	ls.revokedLeases = &atomicLeases{leases: make(map[int64]time.Time)}
	ls.shortLivedLeases = &atomicLeases{leases: make(map[int64]time.Time)}

	ctx, cancel := context.WithCancel(context.Background())
	ls.cancel = cancel
	ls.ctx = ctx

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

		ls.logger.Debug(
			"lease stresser is creating leases",
			zap.String("endpoint", ls.endpoint),
		)
		ls.createLeases()
		ls.logger.Debug(
			"lease stresser created leases",
			zap.String("endpoint", ls.endpoint),
		)

		ls.logger.Debug(
			"lease stresser is dropped leases",
			zap.String("endpoint", ls.endpoint),
		)
		ls.randomlyDropLeases()
		ls.logger.Debug(
			"lease stresser dropped leases",
			zap.String("endpoint", ls.endpoint),
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
}

func (ls *leaseStresser) createLeases() {
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
			leaseID, err := ls.createLeaseWithKeys(TTL)
			if err != nil {
				ls.logger.Debug(
					"createLeaseWithKeys failed",
					zap.String("endpoint", ls.endpoint),
					zap.Error(err),
				)
				return
			}
			ls.aliveLeases.add(leaseID, time.Now())
			// keep track of all the keep lease alive go routines
			ls.aliveWg.Add(1)
			go ls.keepLeaseAlive(leaseID)
		}()
	}
	wg.Wait()
}

func (ls *leaseStresser) createShortLivedLeases() {
	// one round of createLeases() might not create all the short lived leases we want due to falures.
	// thus, we want to create remaining short lived leases in the future round.
	neededLeases := ls.numLeases - len(ls.shortLivedLeases.getLeasesMap())
	var wg sync.WaitGroup
	for i := 0; i < neededLeases; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			leaseID, err := ls.createLeaseWithKeys(TTLShort)
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
		ls.logger.Debug(
			"createLease failed",
			zap.String("endpoint", ls.endpoint),
			zap.Error(err),
		)
		return -1, err
	}

	ls.logger.Debug(
		"createLease created lease",
		zap.String("endpoint", ls.endpoint),
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
				ls.logger.Debug(
					"randomlyDropLease failed",
					zap.String("endpoint", ls.endpoint),
					zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
					zap.Error(err),
				)
				ls.aliveLeases.remove(leaseID)
				return
			}
			if !dropped {
				return
			}
			ls.logger.Debug(
				"randomlyDropLease dropped a lease",
				zap.String("endpoint", ls.endpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
			)
			ls.revokedLeases.add(leaseID, time.Now())
			ls.aliveLeases.remove(leaseID)
		}(l)
	}
	wg.Wait()
}

func (ls *leaseStresser) createLease(ttl int64) (int64, error) {
	resp, err := ls.lc.LeaseGrant(ls.ctx, &pb.LeaseGrantRequest{TTL: ttl})
	if err != nil {
		return -1, err
	}
	return resp.ID, nil
}

func (ls *leaseStresser) keepLeaseAlive(leaseID int64) {
	defer ls.aliveWg.Done()
	ctx, cancel := context.WithCancel(ls.ctx)
	stream, err := ls.lc.LeaseKeepAlive(ctx)
	defer func() { cancel() }()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ls.ctx.Done():
			ls.logger.Debug(
				"keepLeaseAlive context canceled",
				zap.String("endpoint", ls.endpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(ls.ctx.Err()),
			)
			// it is  possible that lease expires at invariant checking phase but not at keepLeaseAlive() phase.
			// this scenerio is possible when alive lease is just about to expire when keepLeaseAlive() exists and expires at invariant checking phase.
			// to circumvent that scenerio, we check each lease before keepalive loop exist to see if it has been renewed in last TTL/2 duration.
			// if it is renewed, this means that invariant checking have at least ttl/2 time before lease exipres which is long enough for the checking to finish.
			// if it is not renewed, we remove the lease from the alive map so that the lease doesn't exipre during invariant checking
			renewTime, ok := ls.aliveLeases.read(leaseID)
			if ok && renewTime.Add(TTL/2*time.Second).Before(time.Now()) {
				ls.aliveLeases.remove(leaseID)
				ls.logger.Debug(
					"keepLeaseAlive lease has not been renewed, dropped it",
					zap.String("endpoint", ls.endpoint),
					zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				)
			}
			return
		}

		if err != nil {
			ls.logger.Debug(
				"keepLeaseAlive lease creates stream error",
				zap.String("endpoint", ls.endpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(err),
			)
			cancel()
			ctx, cancel = context.WithCancel(ls.ctx)
			stream, err = ls.lc.LeaseKeepAlive(ctx)
			cancel()
			continue
		}

		ls.logger.Debug(
			"keepLeaseAlive stream sends lease keepalive request",
			zap.String("endpoint", ls.endpoint),
			zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
		)
		err = stream.Send(&pb.LeaseKeepAliveRequest{ID: leaseID})
		if err != nil {
			ls.logger.Debug(
				"keepLeaseAlive stream failed to send lease keepalive request",
				zap.String("endpoint", ls.endpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(err),
			)
			continue
		}
		leaseRenewTime := time.Now()
		ls.logger.Debug(
			"keepLeaseAlive stream sent lease keepalive request",
			zap.String("endpoint", ls.endpoint),
			zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
		)
		respRC, err := stream.Recv()
		if err != nil {
			ls.logger.Debug(
				"keepLeaseAlive stream failed to receive lease keepalive response",
				zap.String("endpoint", ls.endpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(err),
			)
			continue
		}
		// lease expires after TTL become 0
		// don't send keepalive if the lease has expired
		if respRC.TTL <= 0 {
			ls.logger.Debug(
				"keepLeaseAlive stream received lease keepalive response TTL <= 0",
				zap.String("endpoint", ls.endpoint),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Int64("ttl", respRC.TTL),
			)
			ls.aliveLeases.remove(leaseID)
			return
		}
		// renew lease timestamp only if lease is present
		ls.logger.Debug(
			"keepLeaseAlive renewed a lease",
			zap.String("endpoint", ls.endpoint),
			zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
		)
		ls.aliveLeases.update(leaseID, leaseRenewTime)
	}
}

// attachKeysWithLease function attaches keys to the lease.
// the format of key is the concat of leaseID + '_' + '<order of key creation>'
// e.g 5186835655248304152_0 for first created key and 5186835655248304152_1 for second created key
func (ls *leaseStresser) attachKeysWithLease(leaseID int64) error {
	var txnPuts []*pb.RequestOp
	for j := 0; j < ls.keysPerLease; j++ {
		txnput := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: &pb.PutRequest{Key: []byte(fmt.Sprintf("%d%s%d", leaseID, "_", j)),
			Value: []byte(fmt.Sprintf("bar")), Lease: leaseID}}}
		txnPuts = append(txnPuts, txnput)
	}
	// keep retrying until lease is not found or ctx is being canceled
	for ls.ctx.Err() == nil {
		txn := &pb.TxnRequest{Success: txnPuts}
		_, err := ls.kvc.Txn(ls.ctx, txn)
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
		_, err := ls.lc.LeaseRevoke(ls.ctx, &pb.LeaseRevokeRequest{ID: leaseID})
		if err == nil || rpctypes.Error(err) == rpctypes.ErrLeaseNotFound {
			return true, nil
		}
	}

	ls.logger.Debug(
		"randomlyDropLease error",
		zap.String("endpoint", ls.endpoint),
		zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
		zap.Error(ls.ctx.Err()),
	)
	return false, ls.ctx.Err()
}

func (ls *leaseStresser) Pause() {
	ls.Close()
}

func (ls *leaseStresser) Close() {
	ls.logger.Info(
		"lease stresser is closing",
		zap.String("endpoint", ls.endpoint),
	)
	ls.cancel()
	ls.runWg.Wait()
	ls.aliveWg.Wait()
	ls.conn.Close()
	ls.logger.Info(
		"lease stresser is closed",
		zap.String("endpoint", ls.endpoint),
	)
}

func (ls *leaseStresser) ModifiedKeys() int64 {
	return atomic.LoadInt64(&ls.atomicModifiedKey)
}

func (ls *leaseStresser) Checker() Checker {
	return &leaseChecker{
		logger:   ls.logger,
		endpoint: ls.endpoint,
		ls:       ls,
	}
}
