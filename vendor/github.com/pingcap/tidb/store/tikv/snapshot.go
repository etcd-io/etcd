// Copyright 2015 PingCAP, Inc.
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
	"bytes"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/pingcap/errors"
	pb "github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/metrics"
	"github.com/pingcap/tidb/store/tikv/tikvrpc"
	"github.com/pingcap/tidb/tablecodec"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var (
	_ kv.Snapshot = (*tikvSnapshot)(nil)
)

const (
	scanBatchSize = 256
	batchGetSize  = 5120
)

// tikvSnapshot implements the kv.Snapshot interface.
type tikvSnapshot struct {
	store        *tikvStore
	version      kv.Version
	priority     pb.CommandPri
	notFillCache bool
	syncLog      bool
	keyOnly      bool
	vars         *kv.Variables
}

// newTiKVSnapshot creates a snapshot of an TiKV store.
func newTiKVSnapshot(store *tikvStore, ver kv.Version) *tikvSnapshot {
	return &tikvSnapshot{
		store:    store,
		version:  ver,
		priority: pb.CommandPri_Normal,
		vars:     kv.DefaultVars,
	}
}

func (s *tikvSnapshot) SetPriority(priority int) {
	s.priority = pb.CommandPri(priority)
}

// BatchGet gets all the keys' value from kv-server and returns a map contains key/value pairs.
// The map will not contain nonexistent keys.
func (s *tikvSnapshot) BatchGet(keys []kv.Key) (map[string][]byte, error) {
	metrics.TiKVTxnCmdCounter.WithLabelValues("batch_get").Inc()
	start := time.Now()
	defer func() { metrics.TiKVTxnCmdHistogram.WithLabelValues("batch_get").Observe(time.Since(start).Seconds()) }()

	// We want [][]byte instead of []kv.Key, use some magic to save memory.
	bytesKeys := *(*[][]byte)(unsafe.Pointer(&keys))
	ctx := context.WithValue(context.Background(), txnStartKey, s.version.Ver)
	bo := NewBackoffer(ctx, batchGetMaxBackoff).WithVars(s.vars)

	// Create a map to collect key-values from region servers.
	var mu sync.Mutex
	m := make(map[string][]byte)
	err := s.batchGetKeysByRegions(bo, bytesKeys, func(k, v []byte) {
		if len(v) == 0 {
			return
		}
		mu.Lock()
		m[string(k)] = v
		mu.Unlock()
	})
	if err != nil {
		return nil, errors.Trace(err)
	}

	err = s.store.CheckVisibility(s.version.Ver)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return m, nil
}

func (s *tikvSnapshot) batchGetKeysByRegions(bo *Backoffer, keys [][]byte, collectF func(k, v []byte)) error {
	groups, _, err := s.store.regionCache.GroupKeysByRegion(bo, keys)
	if err != nil {
		return errors.Trace(err)
	}

	metrics.TiKVTxnRegionsNumHistogram.WithLabelValues("snapshot").Observe(float64(len(groups)))

	var batches []batchKeys
	for id, g := range groups {
		batches = appendBatchBySize(batches, id, g, func([]byte) int { return 1 }, batchGetSize)
	}

	if len(batches) == 0 {
		return nil
	}
	if len(batches) == 1 {
		return errors.Trace(s.batchGetSingleRegion(bo, batches[0], collectF))
	}
	ch := make(chan error)
	for _, batch1 := range batches {
		batch := batch1
		go func() {
			backoffer, cancel := bo.Fork()
			defer cancel()
			ch <- s.batchGetSingleRegion(backoffer, batch, collectF)
		}()
	}
	for i := 0; i < len(batches); i++ {
		if e := <-ch; e != nil {
			log.Debugf("snapshot batchGet failed: %v, tid: %d", e, s.version.Ver)
			err = e
		}
	}
	return errors.Trace(err)
}

func (s *tikvSnapshot) batchGetSingleRegion(bo *Backoffer, batch batchKeys, collectF func(k, v []byte)) error {
	sender := NewRegionRequestSender(s.store.regionCache, s.store.client)

	pending := batch.keys
	for {
		req := &tikvrpc.Request{
			Type: tikvrpc.CmdBatchGet,
			BatchGet: &pb.BatchGetRequest{
				Keys:    pending,
				Version: s.version.Ver,
			},
			Context: pb.Context{
				Priority:     s.priority,
				NotFillCache: s.notFillCache,
			},
		}
		resp, err := sender.SendReq(bo, req, batch.region, ReadTimeoutMedium)
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
			err = s.batchGetKeysByRegions(bo, pending, collectF)
			return errors.Trace(err)
		}
		batchGetResp := resp.BatchGet
		if batchGetResp == nil {
			return errors.Trace(ErrBodyMissing)
		}
		var (
			lockedKeys [][]byte
			locks      []*Lock
		)
		for _, pair := range batchGetResp.Pairs {
			keyErr := pair.GetError()
			if keyErr == nil {
				collectF(pair.GetKey(), pair.GetValue())
				continue
			}
			lock, err := extractLockFromKeyErr(keyErr)
			if err != nil {
				return errors.Trace(err)
			}
			lockedKeys = append(lockedKeys, lock.Key)
			locks = append(locks, lock)
		}
		if len(lockedKeys) > 0 {
			ok, err := s.store.lockResolver.ResolveLocks(bo, locks)
			if err != nil {
				return errors.Trace(err)
			}
			if !ok {
				err = bo.Backoff(boTxnLockFast, errors.Errorf("batchGet lockedKeys: %d", len(lockedKeys)))
				if err != nil {
					return errors.Trace(err)
				}
			}
			pending = lockedKeys
			continue
		}
		return nil
	}
}

// Get gets the value for key k from snapshot.
func (s *tikvSnapshot) Get(k kv.Key) ([]byte, error) {
	ctx := context.WithValue(context.Background(), txnStartKey, s.version.Ver)
	val, err := s.get(NewBackoffer(ctx, getMaxBackoff), k)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(val) == 0 {
		return nil, kv.ErrNotExist
	}
	return val, nil
}

func (s *tikvSnapshot) get(bo *Backoffer, k kv.Key) ([]byte, error) {
	sender := NewRegionRequestSender(s.store.regionCache, s.store.client)

	req := &tikvrpc.Request{
		Type: tikvrpc.CmdGet,
		Get: &pb.GetRequest{
			Key:     k,
			Version: s.version.Ver,
		},
		Context: pb.Context{
			Priority:     s.priority,
			NotFillCache: s.notFillCache,
		},
	}
	for {
		loc, err := s.store.regionCache.LocateKey(bo, k)
		if err != nil {
			return nil, errors.Trace(err)
		}
		resp, err := sender.SendReq(bo, req, loc.Region, readTimeoutShort)
		if err != nil {
			return nil, errors.Trace(err)
		}
		regionErr, err := resp.GetRegionError()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if regionErr != nil {
			err = bo.Backoff(BoRegionMiss, errors.New(regionErr.String()))
			if err != nil {
				return nil, errors.Trace(err)
			}
			continue
		}
		cmdGetResp := resp.Get
		if cmdGetResp == nil {
			return nil, errors.Trace(ErrBodyMissing)
		}
		val := cmdGetResp.GetValue()
		if keyErr := cmdGetResp.GetError(); keyErr != nil {
			lock, err := extractLockFromKeyErr(keyErr)
			if err != nil {
				return nil, errors.Trace(err)
			}
			ok, err := s.store.lockResolver.ResolveLocks(bo, []*Lock{lock})
			if err != nil {
				return nil, errors.Trace(err)
			}
			if !ok {
				err = bo.Backoff(boTxnLockFast, errors.New(keyErr.String()))
				if err != nil {
					return nil, errors.Trace(err)
				}
			}
			continue
		}
		return val, nil
	}
}

// Iter return a list of key-value pair after `k`.
func (s *tikvSnapshot) Iter(k kv.Key, upperBound kv.Key) (kv.Iterator, error) {
	scanner, err := newScanner(s, k, upperBound, scanBatchSize)
	return scanner, errors.Trace(err)
}

// IterReverse creates a reversed Iterator positioned on the first entry which key is less than k.
func (s *tikvSnapshot) IterReverse(k kv.Key) (kv.Iterator, error) {
	return nil, kv.ErrNotImplemented
}

func extractLockFromKeyErr(keyErr *pb.KeyError) (*Lock, error) {
	if locked := keyErr.GetLocked(); locked != nil {
		return NewLock(locked), nil
	}
	if keyErr.Conflict != nil {
		err := errors.New(conflictToString(keyErr.Conflict))
		return nil, errors.Annotate(err, txnRetryableMark)
	}
	if keyErr.Retryable != "" {
		err := errors.Errorf("tikv restarts txn: %s", keyErr.GetRetryable())
		log.Debug(err)
		return nil, errors.Annotate(err, txnRetryableMark)
	}
	if keyErr.Abort != "" {
		err := errors.Errorf("tikv aborts txn: %s", keyErr.GetAbort())
		log.Warn(err)
		return nil, errors.Trace(err)
	}
	return nil, errors.Errorf("unexpected KeyError: %s", keyErr.String())
}

func conflictToString(conflict *pb.WriteConflict) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "WriteConflict: startTS=%d, conflictTS=%d, key=", conflict.StartTs, conflict.ConflictTs)
	prettyWriteKey(&buf, conflict.Key)
	buf.WriteString(" primary=")
	prettyWriteKey(&buf, conflict.Primary)
	return buf.String()
}

func prettyWriteKey(buf *bytes.Buffer, key []byte) {
	tableID, indexID, indexValues, err := tablecodec.DecodeIndexKey(key)
	if err == nil {
		fmt.Fprintf(buf, "{tableID=%d, indexID=%d, indexValues={", tableID, indexID)
		for _, v := range indexValues {
			fmt.Fprintf(buf, "%s, ", v)
		}
		buf.WriteString("}}")
		return
	}

	tableID, handle, err := tablecodec.DecodeRecordKey(key)
	if err == nil {
		fmt.Fprintf(buf, "{tableID=%d, handle=%d}", tableID, handle)
		return
	}

	fmt.Fprintf(buf, "%#v", key)
}
