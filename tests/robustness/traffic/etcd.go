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

	"golang.org/x/time/rate"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/identity"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

var (
	LowTraffic = Config{
		Name:        "LowTraffic",
		minimalQPS:  100,
		maximalQPS:  200,
		clientCount: 8,
		Traffic: etcdTraffic{
			keyCount:     10,
			leaseTTL:     DefaultLeaseTTL,
			largePutSize: 32769,
			operations: []choiceWeight[etcdRequestType]{
				{choice: Get, weight: 50},
				{choice: Put, weight: 23},
				{choice: LargePut, weight: 2},
				{choice: Delete, weight: 5},
				{choice: MultiOpTxn, weight: 5},
				{choice: PutWithLease, weight: 5},
				{choice: LeaseRevoke, weight: 5},
				{choice: CompareAndSet, weight: 5},
			},
		},
	}
	HighTraffic = Config{
		Name:        "HighTraffic",
		minimalQPS:  200,
		maximalQPS:  1000,
		clientCount: 12,
		Traffic: etcdTraffic{
			keyCount:     10,
			largePutSize: 32769,
			leaseTTL:     DefaultLeaseTTL,
			operations: []choiceWeight[etcdRequestType]{
				{choice: Get, weight: 50},
				{choice: Put, weight: 40},
				{choice: MultiOpTxn, weight: 5},
				{choice: LargePut, weight: 5},
			},
		},
	}
)

type etcdTraffic struct {
	keyCount     int
	operations   []choiceWeight[etcdRequestType]
	leaseTTL     int64
	largePutSize int
}

func (t etcdTraffic) ExpectUniqueRevision() bool {
	return false
}

type etcdRequestType string

const (
	Get           etcdRequestType = "get"
	Put           etcdRequestType = "put"
	LargePut      etcdRequestType = "largePut"
	Delete        etcdRequestType = "delete"
	MultiOpTxn    etcdRequestType = "multiOpTxn"
	PutWithLease  etcdRequestType = "putWithLease"
	LeaseRevoke   etcdRequestType = "leaseRevoke"
	CompareAndSet etcdRequestType = "compareAndSet"
	Defragment    etcdRequestType = "defragment"
)

func (t etcdTraffic) Run(ctx context.Context, c *RecordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage, finish <-chan struct{}) {
	lastOperationSucceeded := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-finish:
			return
		default:
		}
		key := fmt.Sprintf("%d", rand.Int()%t.keyCount)
		// Avoid multiple failed writes in a row
		if !lastOperationSucceeded {
			_, err := t.Read(ctx, c, key)
			if err != nil {
				continue
			}
			limiter.Wait(ctx)
		}
		err := t.RandomOperation(ctx, c, limiter, key, ids, lm)
		lastOperationSucceeded = err == nil
		if err != nil {
			continue
		}
		limiter.Wait(ctx)
	}
}

func (t etcdTraffic) Read(ctx context.Context, c *RecordingClient, key string) (*mvccpb.KeyValue, error) {
	getCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	resp, err := c.Get(getCtx, key)
	cancel()
	return resp, err
}

func (t etcdTraffic) RandomOperation(ctx context.Context, c *RecordingClient, limiter *rate.Limiter, key string, id identity.Provider, lm identity.LeaseIdStorage) error {
	opCtx, cancel := context.WithTimeout(ctx, RequestTimeout)

	var err error
	switch pickRandom(t.operations) {
	case Get:
		_, err = c.Get(opCtx, key)
	case Put:
		err = c.Put(opCtx, key, fmt.Sprintf("%d", id.NewRequestId()))
	case LargePut:
		err = c.Put(opCtx, key, randString(t.largePutSize))
	case Delete:
		err = c.Delete(opCtx, key)
	case MultiOpTxn:
		_, err = c.Txn(opCtx, nil, t.pickMultiTxnOps(id), nil)
	case CompareAndSet:
		var kv *mvccpb.KeyValue
		kv, err = c.Get(opCtx, key)
		if err == nil {
			limiter.Wait(ctx)
			var expectedRevision int64
			if kv != nil {
				expectedRevision = kv.ModRevision
			}
			txnCtx, txnCancel := context.WithTimeout(ctx, RequestTimeout)
			_, err = c.Txn(txnCtx, []clientv3.Cmp{clientv3.Compare(clientv3.ModRevision(key), "=", expectedRevision)}, []clientv3.Op{clientv3.OpPut(key, fmt.Sprintf("%d", id.NewRequestId()))}, nil)
			txnCancel()
		}
	case PutWithLease:
		leaseId := lm.LeaseId(c.id)
		if leaseId == 0 {
			leaseId, err = c.LeaseGrant(opCtx, t.leaseTTL)
			if err == nil {
				lm.AddLeaseId(c.id, leaseId)
				limiter.Wait(ctx)
			}
		}
		if leaseId != 0 {
			putCtx, putCancel := context.WithTimeout(ctx, RequestTimeout)
			err = c.PutWithLease(putCtx, key, fmt.Sprintf("%d", id.NewRequestId()), leaseId)
			putCancel()
		}
	case LeaseRevoke:
		leaseId := lm.LeaseId(c.id)
		if leaseId != 0 {
			err = c.LeaseRevoke(opCtx, leaseId)
			//if LeaseRevoke has failed, do not remove the mapping.
			if err == nil {
				lm.RemoveLeaseId(c.id)
			}
		}
	case Defragment:
		err = c.Defragment(opCtx)
	default:
		panic("invalid choice")
	}
	cancel()
	return err
}

func (t etcdTraffic) pickMultiTxnOps(ids identity.Provider) (ops []clientv3.Op) {
	keys := rand.Perm(t.keyCount)
	opTypes := make([]model.OperationType, 4)

	atLeastOnePut := false
	for i := 0; i < MultiOpTxnOpCount; i++ {
		opTypes[i] = t.pickOperationType()
		if opTypes[i] == model.Put {
			atLeastOnePut = true
		}
	}
	// Ensure at least one put to make operation unique
	if !atLeastOnePut {
		opTypes[0] = model.Put
	}

	for i, opType := range opTypes {
		key := fmt.Sprintf("%d", keys[i])
		switch opType {
		case model.Range:
			ops = append(ops, clientv3.OpGet(key))
		case model.Put:
			value := fmt.Sprintf("%d", ids.NewRequestId())
			ops = append(ops, clientv3.OpPut(key, value))
		case model.Delete:
			ops = append(ops, clientv3.OpDelete(key))
		default:
			panic("unsuported choice type")
		}
	}
	return ops
}

func (t etcdTraffic) pickOperationType() model.OperationType {
	roll := rand.Int() % 100
	if roll < 10 {
		return model.Delete
	}
	if roll < 50 {
		return model.Range
	}
	return model.Put
}
