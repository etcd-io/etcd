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
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/time/rate"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/client"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/random"
)

var (
	EtcdPutDeleteLease Traffic = etcdTraffic{
		keyCount:     10,
		leaseTTL:     DefaultLeaseTTL,
		largePutSize: 32769,
		// Please keep the sum of weights equal 100.
		requests: []random.ChoiceWeight[etcdRequestType]{
			{Choice: Get, Weight: 15},
			{Choice: List, Weight: 15},
			{Choice: StaleGet, Weight: 10},
			{Choice: StaleList, Weight: 10},
			{Choice: Delete, Weight: 5},
			{Choice: MultiOpTxn, Weight: 5},
			{Choice: PutWithLease, Weight: 5},
			{Choice: LeaseRevoke, Weight: 5},
			{Choice: CompareAndSet, Weight: 5},
			{Choice: Put, Weight: 20},
			{Choice: LargePut, Weight: 5},
		},
	}
	EtcdPut Traffic = etcdTraffic{
		keyCount:     10,
		largePutSize: 32769,
		leaseTTL:     DefaultLeaseTTL,
		// Please keep the sum of weights equal 100.
		requests: []random.ChoiceWeight[etcdRequestType]{
			{Choice: Get, Weight: 15},
			{Choice: List, Weight: 15},
			{Choice: StaleGet, Weight: 10},
			{Choice: StaleList, Weight: 10},
			{Choice: MultiOpTxn, Weight: 5},
			{Choice: LargePut, Weight: 5},
			{Choice: Put, Weight: 40},
		},
	}
	EtcdDelete Traffic = etcdTraffic{
		keyCount:     10,
		largePutSize: 32769,
		leaseTTL:     DefaultLeaseTTL,
		// Please keep the sum of weights equal 100.
		requests: []random.ChoiceWeight[etcdRequestType]{
			{Choice: Put, Weight: 50},
			{Choice: Delete, Weight: 50},
		},
	}
)

type etcdTraffic struct {
	keyCount     int
	requests     []random.ChoiceWeight[etcdRequestType]
	leaseTTL     int64
	largePutSize int
}

func (t etcdTraffic) ExpectUniqueRevision() bool {
	return false
}

type etcdRequestType string

const (
	Get           etcdRequestType = "get"
	StaleGet      etcdRequestType = "staleGet"
	List          etcdRequestType = "list"
	StaleList     etcdRequestType = "staleList"
	Put           etcdRequestType = "put"
	LargePut      etcdRequestType = "largePut"
	Delete        etcdRequestType = "delete"
	MultiOpTxn    etcdRequestType = "multiOpTxn"
	PutWithLease  etcdRequestType = "putWithLease"
	LeaseRevoke   etcdRequestType = "leaseRevoke"
	CompareAndSet etcdRequestType = "compareAndSet"
	Defragment    etcdRequestType = "defragment"
)

func (t etcdTraffic) Name() string {
	return "Etcd"
}

func (t etcdTraffic) RunTrafficLoop(ctx context.Context, c *client.RecordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIDStorage, nonUniqueWriteLimiter ConcurrencyLimiter, finish <-chan struct{}) {
	lastOperationSucceeded := true
	var lastRev int64
	var requestType etcdRequestType
	client := etcdTrafficClient{
		etcdTraffic:  t,
		keyPrefix:    "key",
		client:       c,
		limiter:      limiter,
		idProvider:   ids,
		leaseStorage: lm,
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-finish:
			return
		default:
		}
		shouldReturn := false

		// Avoid multiple failed writes in a row
		if lastOperationSucceeded {
			choices := t.requests
			if shouldReturn = nonUniqueWriteLimiter.Take(); !shouldReturn {
				choices = filterOutNonUniqueEtcdWrites(choices)
			}
			requestType = random.PickRandom(choices)
		} else {
			requestType = Get
		}
		rev, err := client.Request(ctx, requestType, lastRev)
		if shouldReturn {
			nonUniqueWriteLimiter.Return()
		}
		lastOperationSucceeded = err == nil
		if err != nil {
			continue
		}
		if rev != 0 {
			lastRev = rev
		}
		limiter.Wait(ctx)
	}
}

func (t etcdTraffic) RunCompactLoop(ctx context.Context, c *client.RecordingClient, period time.Duration, finish <-chan struct{}) {
	var lastRev int64 = 2
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-finish:
			return
		case <-ticker.C:
		}
		statusCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
		resp, err := c.Status(statusCtx, c.Endpoints()[0])
		cancel()
		if err != nil {
			continue
		}

		// Range allows for both revision has been compacted and future revision errors
		compactRev := random.RandRange(lastRev, resp.Header.Revision+5)
		_, err = c.Compact(ctx, compactRev)
		if err != nil {
			continue
		}
		lastRev = compactRev
	}
}

func filterOutNonUniqueEtcdWrites(choices []random.ChoiceWeight[etcdRequestType]) (resp []random.ChoiceWeight[etcdRequestType]) {
	for _, choice := range choices {
		if choice.Choice != Delete && choice.Choice != LeaseRevoke {
			resp = append(resp, choice)
		}
	}
	return resp
}

type etcdTrafficClient struct {
	etcdTraffic
	keyPrefix    string
	client       *client.RecordingClient
	limiter      *rate.Limiter
	idProvider   identity.Provider
	leaseStorage identity.LeaseIDStorage
}

func (c etcdTrafficClient) Request(ctx context.Context, request etcdRequestType, lastRev int64) (rev int64, err error) {
	opCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()

	var limit int64
	switch request {
	case StaleGet:
		var resp *clientv3.GetResponse
		resp, err = c.client.Get(opCtx, c.randomKey(), clientv3.WithRev(lastRev))
		if err == nil {
			rev = resp.Header.Revision
		}
	case Get:
		var resp *clientv3.GetResponse
		resp, err = c.client.Get(opCtx, c.randomKey(), clientv3.WithRev(0))
		if err == nil {
			rev = resp.Header.Revision
		}
	case List:
		var resp *clientv3.GetResponse
		resp, err = c.client.Range(ctx, c.keyPrefix, clientv3.GetPrefixRangeEnd(c.keyPrefix), 0, limit)
		if resp != nil {
			rev = resp.Header.Revision
		}
	case StaleList:
		var resp *clientv3.GetResponse
		resp, err = c.client.Range(ctx, c.keyPrefix, clientv3.GetPrefixRangeEnd(c.keyPrefix), lastRev, limit)
		if resp != nil {
			rev = resp.Header.Revision
		}
	case Put:
		var resp *clientv3.PutResponse
		resp, err = c.client.Put(opCtx, c.randomKey(), fmt.Sprintf("%d", c.idProvider.NewRequestID()))
		if resp != nil {
			rev = resp.Header.Revision
		}
	case LargePut:
		var resp *clientv3.PutResponse
		resp, err = c.client.Put(opCtx, c.randomKey(), random.RandString(c.largePutSize))
		if resp != nil {
			rev = resp.Header.Revision
		}
	case Delete:
		var resp *clientv3.DeleteResponse
		resp, err = c.client.Delete(opCtx, c.randomKey())
		if resp != nil {
			rev = resp.Header.Revision
		}
	case MultiOpTxn:
		var resp *clientv3.TxnResponse
		resp, err = c.client.Txn(opCtx).Then(
			c.pickMultiTxnOps()...,
		).Commit()
		if resp != nil {
			rev = resp.Header.Revision
		}
	case CompareAndSet:
		var kv *mvccpb.KeyValue
		key := c.randomKey()
		var resp *clientv3.GetResponse
		resp, err = c.client.Get(opCtx, key, clientv3.WithRev(0))
		if err == nil {
			rev = resp.Header.Revision
			if len(resp.Kvs) == 1 {
				kv = resp.Kvs[0]
			}
			c.limiter.Wait(ctx)
			var expectedRevision int64
			if kv != nil {
				expectedRevision = kv.ModRevision
			}
			txnCtx, txnCancel := context.WithTimeout(ctx, RequestTimeout)
			var resp *clientv3.TxnResponse
			resp, err = c.client.Txn(txnCtx).If(
				clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision),
			).Then(
				clientv3.OpPut(key, fmt.Sprintf("%d", c.idProvider.NewRequestID())),
			).Commit()
			txnCancel()
			if resp != nil {
				rev = resp.Header.Revision
			}
		}
	case PutWithLease:
		leaseID := c.leaseStorage.LeaseID(c.client.ID)
		if leaseID == 0 {
			var resp *clientv3.LeaseGrantResponse
			resp, err = c.client.LeaseGrant(opCtx, c.leaseTTL)
			if resp != nil {
				leaseID = int64(resp.ID)
				rev = resp.ResponseHeader.Revision
			}
			if err == nil {
				c.leaseStorage.AddLeaseID(c.client.ID, leaseID)
				c.limiter.Wait(ctx)
			}
		}
		if leaseID != 0 {
			putCtx, putCancel := context.WithTimeout(ctx, RequestTimeout)
			var resp *clientv3.PutResponse
			resp, err = c.client.PutWithLease(putCtx, c.randomKey(), fmt.Sprintf("%d", c.idProvider.NewRequestID()), leaseID)
			putCancel()
			if resp != nil {
				rev = resp.Header.Revision
			}
		}
	case LeaseRevoke:
		leaseID := c.leaseStorage.LeaseID(c.client.ID)
		if leaseID != 0 {
			var resp *clientv3.LeaseRevokeResponse
			resp, err = c.client.LeaseRevoke(opCtx, leaseID)
			// if LeaseRevoke has failed, do not remove the mapping.
			if err == nil {
				c.leaseStorage.RemoveLeaseID(c.client.ID)
			}
			if resp != nil {
				rev = resp.Header.Revision
			}
		}
	case Defragment:
		var resp *clientv3.DefragmentResponse
		resp, err = c.client.Defragment(opCtx)
		if resp != nil {
			rev = resp.Header.Revision
		}
	default:
		panic("invalid choice")
	}
	return rev, err
}

func (c etcdTrafficClient) pickMultiTxnOps() (ops []clientv3.Op) {
	keys := rand.Perm(c.keyCount)
	opTypes := make([]model.OperationType, 4)

	atLeastOnePut := false
	for i := 0; i < MultiOpTxnOpCount; i++ {
		opTypes[i] = c.pickOperationType()
		if opTypes[i] == model.PutOperation {
			atLeastOnePut = true
		}
	}
	// Ensure at least one put to make operation unique
	if !atLeastOnePut {
		opTypes[0] = model.PutOperation
	}

	for i, opType := range opTypes {
		key := c.key(keys[i])
		switch opType {
		case model.RangeOperation:
			ops = append(ops, clientv3.OpGet(key))
		case model.PutOperation:
			value := fmt.Sprintf("%d", c.idProvider.NewRequestID())
			ops = append(ops, clientv3.OpPut(key, value))
		case model.DeleteOperation:
			ops = append(ops, clientv3.OpDelete(key))
		default:
			panic("unsuported choice type")
		}
	}
	return ops
}

func (c etcdTrafficClient) randomKey() string {
	return c.key(rand.Int())
}

func (c etcdTrafficClient) key(i int) string {
	return fmt.Sprintf("%s%d", c.keyPrefix, i%c.keyCount)
}

func (t etcdTraffic) pickOperationType() model.OperationType {
	roll := rand.Int() % 100
	if roll < 10 {
		return model.DeleteOperation
	}
	if roll < 50 {
		return model.RangeOperation
	}
	return model.PutOperation
}
