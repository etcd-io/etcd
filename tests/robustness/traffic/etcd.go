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
		traffic: etcdTraffic{
			keyCount:     10,
			leaseTTL:     DefaultLeaseTTL,
			largePutSize: 32769,
			writeChoices: []choiceWeight[etcdRequestType]{
				{choice: Put, weight: 45},
				{choice: LargePut, weight: 5},
				{choice: Delete, weight: 10},
				{choice: MultiOpTxn, weight: 10},
				{choice: PutWithLease, weight: 10},
				{choice: LeaseRevoke, weight: 10},
				{choice: CompareAndSet, weight: 10},
			},
		},
	}
	HighTraffic = Config{
		Name:        "HighTraffic",
		minimalQPS:  200,
		maximalQPS:  1000,
		clientCount: 12,
		traffic: etcdTraffic{
			keyCount:     10,
			largePutSize: 32769,
			leaseTTL:     DefaultLeaseTTL,
			writeChoices: []choiceWeight[etcdRequestType]{
				{choice: Put, weight: 85},
				{choice: MultiOpTxn, weight: 10},
				{choice: LargePut, weight: 5},
			},
		},
	}
)

type etcdTraffic struct {
	keyCount     int
	writeChoices []choiceWeight[etcdRequestType]
	leaseTTL     int64
	largePutSize int
}

type etcdRequestType string

const (
	Put           etcdRequestType = "put"
	LargePut      etcdRequestType = "largePut"
	Delete        etcdRequestType = "delete"
	MultiOpTxn    etcdRequestType = "multiOpTxn"
	PutWithLease  etcdRequestType = "putWithLease"
	LeaseRevoke   etcdRequestType = "leaseRevoke"
	CompareAndSet etcdRequestType = "compareAndSet"
	Defragment    etcdRequestType = "defragment"
)

func (t etcdTraffic) Run(ctx context.Context, clientId int, c *RecordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage, finish <-chan struct{}) {

	for {
		select {
		case <-ctx.Done():
			return
		case <-finish:
			return
		default:
		}
		key := fmt.Sprintf("%d", rand.Int()%t.keyCount)
		// Execute one read per one write to avoid operation history include too many failed writes when etcd is down.
		resp, err := t.Read(ctx, c, key)
		if err != nil {
			continue
		}
		limiter.Wait(ctx)
		err = t.Write(ctx, c, limiter, key, ids, lm, clientId, resp)
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

func (t etcdTraffic) Write(ctx context.Context, c *RecordingClient, limiter *rate.Limiter, key string, id identity.Provider, lm identity.LeaseIdStorage, cid int, lastValues *mvccpb.KeyValue) error {
	writeCtx, cancel := context.WithTimeout(ctx, RequestTimeout)

	var err error
	switch etcdRequestType(pickRandom(t.writeChoices)) {
	case Put:
		err = c.Put(writeCtx, key, fmt.Sprintf("%d", id.RequestId()))
	case LargePut:
		err = c.Put(writeCtx, key, randString(t.largePutSize))
	case Delete:
		err = c.Delete(writeCtx, key)
	case MultiOpTxn:
		err = c.Txn(writeCtx, nil, t.pickMultiTxnOps(id))
	case CompareAndSet:
		var expectRevision int64
		if lastValues != nil {
			expectRevision = lastValues.ModRevision
		}
		err = c.CompareRevisionAndPut(writeCtx, key, fmt.Sprintf("%d", id.RequestId()), expectRevision)
	case PutWithLease:
		leaseId := lm.LeaseId(cid)
		if leaseId == 0 {
			leaseId, err = c.LeaseGrant(writeCtx, t.leaseTTL)
			if err == nil {
				lm.AddLeaseId(cid, leaseId)
				limiter.Wait(ctx)
			}
		}
		if leaseId != 0 {
			putCtx, putCancel := context.WithTimeout(ctx, RequestTimeout)
			err = c.PutWithLease(putCtx, key, fmt.Sprintf("%d", id.RequestId()), leaseId)
			putCancel()
		}
	case LeaseRevoke:
		leaseId := lm.LeaseId(cid)
		if leaseId != 0 {
			err = c.LeaseRevoke(writeCtx, leaseId)
			//if LeaseRevoke has failed, do not remove the mapping.
			if err == nil {
				lm.RemoveLeaseId(cid)
			}
		}
	case Defragment:
		err = c.Defragment(writeCtx)
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
			value := fmt.Sprintf("%d", ids.RequestId())
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
