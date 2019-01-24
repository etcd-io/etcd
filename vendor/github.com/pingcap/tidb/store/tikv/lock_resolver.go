// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package tikv

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/pd/client"
	"github.com/pingcap/tidb/config"
	"github.com/pingcap/tidb/metrics"
	"github.com/pingcap/tidb/store/tikv/tikvrpc"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// ResolvedCacheSize is max number of cached txn status.
const ResolvedCacheSize = 2048

// LockResolver resolves locks and also caches resolved txn status.
type LockResolver struct {
	store Storage
	mu    struct {
		sync.RWMutex
		// resolved caches resolved txns (FIFO, txn id -> txnStatus).
		resolved       map[uint64]TxnStatus
		recentResolved *list.List
	}
}

func newLockResolver(store Storage) *LockResolver {
	r := &LockResolver{
		store: store,
	}
	r.mu.resolved = make(map[uint64]TxnStatus)
	r.mu.recentResolved = list.New()
	return r
}

// NewLockResolver is exported for other pkg to use, suppress unused warning.
var _ = NewLockResolver

// NewLockResolver creates a LockResolver.
// It is exported for other pkg to use. For instance, binlog service needs
// to determine a transaction's commit state.
func NewLockResolver(etcdAddrs []string, security config.Security) (*LockResolver, error) {
	pdCli, err := pd.NewClient(etcdAddrs, pd.SecurityOption{
		CAPath:   security.ClusterSSLCA,
		CertPath: security.ClusterSSLCert,
		KeyPath:  security.ClusterSSLKey,
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	uuid := fmt.Sprintf("tikv-%v", pdCli.GetClusterID(context.TODO()))

	tlsConfig, err := security.ToTLSConfig()
	if err != nil {
		return nil, errors.Trace(err)
	}

	spkv, err := NewEtcdSafePointKV(etcdAddrs, tlsConfig)
	if err != nil {
		return nil, errors.Trace(err)
	}

	s, err := newTikvStore(uuid, &codecPDClient{pdCli}, spkv, newRPCClient(security), false)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return s.lockResolver, nil
}

// TxnStatus represents a txn's final status. It should be Commit or Rollback.
type TxnStatus uint64

// IsCommitted returns true if the txn's final status is Commit.
func (s TxnStatus) IsCommitted() bool { return s > 0 }

// CommitTS returns the txn's commitTS. It is valid iff `IsCommitted` is true.
func (s TxnStatus) CommitTS() uint64 { return uint64(s) }

// By default, locks after 3000ms is considered unusual (the client created the
// lock might be dead). Other client may cleanup this kind of lock.
// For locks created recently, we will do backoff and retry.
var defaultLockTTL uint64 = 3000

// TODO: Consider if it's appropriate.
var maxLockTTL uint64 = 120000

// ttl = ttlFactor * sqrt(writeSizeInMiB)
var ttlFactor = 6000

// Lock represents a lock from tikv server.
type Lock struct {
	Key     []byte
	Primary []byte
	TxnID   uint64
	TTL     uint64
}

// NewLock creates a new *Lock.
func NewLock(l *kvrpcpb.LockInfo) *Lock {
	ttl := l.GetLockTtl()
	if ttl == 0 {
		ttl = defaultLockTTL
	}
	return &Lock{
		Key:     l.GetKey(),
		Primary: l.GetPrimaryLock(),
		TxnID:   l.GetLockVersion(),
		TTL:     ttl,
	}
}

func (lr *LockResolver) saveResolved(txnID uint64, status TxnStatus) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if _, ok := lr.mu.resolved[txnID]; ok {
		return
	}
	lr.mu.resolved[txnID] = status
	lr.mu.recentResolved.PushBack(txnID)
	if len(lr.mu.resolved) > ResolvedCacheSize {
		front := lr.mu.recentResolved.Front()
		delete(lr.mu.resolved, front.Value.(uint64))
		lr.mu.recentResolved.Remove(front)
	}
}

func (lr *LockResolver) getResolved(txnID uint64) (TxnStatus, bool) {
	lr.mu.RLock()
	defer lr.mu.RUnlock()

	s, ok := lr.mu.resolved[txnID]
	return s, ok
}

// BatchResolveLocks resolve locks in a batch
func (lr *LockResolver) BatchResolveLocks(bo *Backoffer, locks []*Lock, loc RegionVerID) (bool, error) {
	if len(locks) == 0 {
		return true, nil
	}

	metrics.TiKVLockResolverCounter.WithLabelValues("batch_resolve").Inc()

	var expiredLocks []*Lock
	for _, l := range locks {
		if lr.store.GetOracle().IsExpired(l.TxnID, l.TTL) {
			metrics.TiKVLockResolverCounter.WithLabelValues("expired").Inc()
			expiredLocks = append(expiredLocks, l)
		} else {
			metrics.TiKVLockResolverCounter.WithLabelValues("not_expired").Inc()
		}
	}
	if len(expiredLocks) != len(locks) {
		log.Errorf("BatchResolveLocks: get %d Locks, but only %d are expired, maybe safe point is wrong!", len(locks), len(expiredLocks))
		return false, nil
	}

	startTime := time.Now()
	txnInfos := make(map[uint64]uint64)
	for _, l := range expiredLocks {
		if _, ok := txnInfos[l.TxnID]; ok {
			continue
		}

		status, err := lr.getTxnStatus(bo, l.TxnID, l.Primary)
		if err != nil {
			return false, errors.Trace(err)
		}
		txnInfos[l.TxnID] = uint64(status)
	}
	log.Infof("BatchResolveLocks: it took %v to lookup %v txn status", time.Since(startTime), len(txnInfos))

	var listTxnInfos []*kvrpcpb.TxnInfo
	for txnID, status := range txnInfos {
		listTxnInfos = append(listTxnInfos, &kvrpcpb.TxnInfo{
			Txn:    txnID,
			Status: status,
		})
	}

	req := &tikvrpc.Request{
		Type: tikvrpc.CmdResolveLock,
		ResolveLock: &kvrpcpb.ResolveLockRequest{
			TxnInfos: listTxnInfos,
		},
	}
	startTime = time.Now()
	resp, err := lr.store.SendReq(bo, req, loc, readTimeoutShort)
	if err != nil {
		return false, errors.Trace(err)
	}

	regionErr, err := resp.GetRegionError()
	if err != nil {
		return false, errors.Trace(err)
	}

	if regionErr != nil {
		err = bo.Backoff(BoRegionMiss, errors.New(regionErr.String()))
		if err != nil {
			return false, errors.Trace(err)
		}
		return false, nil
	}

	cmdResp := resp.ResolveLock
	if cmdResp == nil {
		return false, errors.Trace(ErrBodyMissing)
	}
	if keyErr := cmdResp.GetError(); keyErr != nil {
		return false, errors.Errorf("unexpected resolve err: %s", keyErr)
	}

	log.Infof("BatchResolveLocks: it took %v to resolve %v locks in a batch.", time.Since(startTime), len(expiredLocks))
	return true, nil
}

// ResolveLocks tries to resolve Locks. The resolving process is in 3 steps:
// 1) Use the `lockTTL` to pick up all expired locks. Only locks that are too
//    old are considered orphan locks and will be handled later. If all locks
//    are expired then all locks will be resolved so the returned `ok` will be
//    true, otherwise caller should sleep a while before retry.
// 2) For each lock, query the primary key to get txn(which left the lock)'s
//    commit status.
// 3) Send `ResolveLock` cmd to the lock's region to resolve all locks belong to
//    the same transaction.
func (lr *LockResolver) ResolveLocks(bo *Backoffer, locks []*Lock) (ok bool, err error) {
	if len(locks) == 0 {
		return true, nil
	}

	metrics.TiKVLockResolverCounter.WithLabelValues("resolve").Inc()

	var expiredLocks []*Lock
	for _, l := range locks {
		if lr.store.GetOracle().IsExpired(l.TxnID, l.TTL) {
			metrics.TiKVLockResolverCounter.WithLabelValues("expired").Inc()
			expiredLocks = append(expiredLocks, l)
		} else {
			metrics.TiKVLockResolverCounter.WithLabelValues("not_expired").Inc()
		}
	}
	if len(expiredLocks) == 0 {
		return false, nil
	}

	// TxnID -> []Region, record resolved Regions.
	// TODO: Maybe put it in LockResolver and share by all txns.
	cleanTxns := make(map[uint64]map[RegionVerID]struct{})
	for _, l := range expiredLocks {
		status, err := lr.getTxnStatus(bo, l.TxnID, l.Primary)
		if err != nil {
			return false, errors.Trace(err)
		}

		cleanRegions := cleanTxns[l.TxnID]
		if cleanRegions == nil {
			cleanRegions = make(map[RegionVerID]struct{})
			cleanTxns[l.TxnID] = cleanRegions
		}

		err = lr.resolveLock(bo, l, status, cleanRegions)
		if err != nil {
			return false, errors.Trace(err)
		}
	}
	return len(expiredLocks) == len(locks), nil
}

// GetTxnStatus queries tikv-server for a txn's status (commit/rollback).
// If the primary key is still locked, it will launch a Rollback to abort it.
// To avoid unnecessarily aborting too many txns, it is wiser to wait a few
// seconds before calling it after Prewrite.
func (lr *LockResolver) GetTxnStatus(txnID uint64, primary []byte) (TxnStatus, error) {
	bo := NewBackoffer(context.Background(), cleanupMaxBackoff)
	status, err := lr.getTxnStatus(bo, txnID, primary)
	return status, errors.Trace(err)
}

func (lr *LockResolver) getTxnStatus(bo *Backoffer, txnID uint64, primary []byte) (TxnStatus, error) {
	if s, ok := lr.getResolved(txnID); ok {
		return s, nil
	}

	metrics.TiKVLockResolverCounter.WithLabelValues("query_txn_status").Inc()

	var status TxnStatus
	req := &tikvrpc.Request{
		Type: tikvrpc.CmdCleanup,
		Cleanup: &kvrpcpb.CleanupRequest{
			Key:          primary,
			StartVersion: txnID,
		},
	}
	for {
		loc, err := lr.store.GetRegionCache().LocateKey(bo, primary)
		if err != nil {
			return status, errors.Trace(err)
		}
		resp, err := lr.store.SendReq(bo, req, loc.Region, readTimeoutShort)
		if err != nil {
			return status, errors.Trace(err)
		}
		regionErr, err := resp.GetRegionError()
		if err != nil {
			return status, errors.Trace(err)
		}
		if regionErr != nil {
			err = bo.Backoff(BoRegionMiss, errors.New(regionErr.String()))
			if err != nil {
				return status, errors.Trace(err)
			}
			continue
		}
		cmdResp := resp.Cleanup
		if cmdResp == nil {
			return status, errors.Trace(ErrBodyMissing)
		}
		if keyErr := cmdResp.GetError(); keyErr != nil {
			err = errors.Errorf("unexpected cleanup err: %s, tid: %v", keyErr, txnID)
			log.Error(err)
			return status, err
		}
		if cmdResp.CommitVersion != 0 {
			status = TxnStatus(cmdResp.GetCommitVersion())
			metrics.TiKVLockResolverCounter.WithLabelValues("query_txn_status_committed").Inc()
		} else {
			metrics.TiKVLockResolverCounter.WithLabelValues("query_txn_status_rolled_back").Inc()
		}
		lr.saveResolved(txnID, status)
		return status, nil
	}
}

func (lr *LockResolver) resolveLock(bo *Backoffer, l *Lock, status TxnStatus, cleanRegions map[RegionVerID]struct{}) error {
	metrics.TiKVLockResolverCounter.WithLabelValues("query_resolve_locks").Inc()
	for {
		loc, err := lr.store.GetRegionCache().LocateKey(bo, l.Key)
		if err != nil {
			return errors.Trace(err)
		}
		if _, ok := cleanRegions[loc.Region]; ok {
			return nil
		}
		req := &tikvrpc.Request{
			Type: tikvrpc.CmdResolveLock,
			ResolveLock: &kvrpcpb.ResolveLockRequest{
				StartVersion: l.TxnID,
			},
		}
		if status.IsCommitted() {
			req.ResolveLock.CommitVersion = status.CommitTS()
		}
		resp, err := lr.store.SendReq(bo, req, loc.Region, readTimeoutShort)
		if err != nil {
			return errors.Trace(err)
		}
		regionErr, err := resp.GetRegionError()
		if err != nil {
			return errors.Trace(err)
		}
		if regionErr != nil {
			err = bo.Backoff(BoRegionMiss, errors.New(regionErr.String()))
			if err != nil {
				return errors.Trace(err)
			}
			continue
		}
		cmdResp := resp.ResolveLock
		if cmdResp == nil {
			return errors.Trace(ErrBodyMissing)
		}
		if keyErr := cmdResp.GetError(); keyErr != nil {
			err = errors.Errorf("unexpected resolve err: %s, lock: %v", keyErr, l)
			log.Error(err)
			return err
		}
		cleanRegions[loc.Region] = struct{}{}
		return nil
	}
}
