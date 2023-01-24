// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/time/rate"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/tests/v3/linearizability/identity"
)

var (
	DefaultLeaseTTL int64 = 7200
	RequestTimeout        = 40 * time.Millisecond
)

type TrafficRequestType string

const (
	Get           TrafficRequestType = "get"
	Put           TrafficRequestType = "put"
	Delete        TrafficRequestType = "delete"
	PutWithLease  TrafficRequestType = "putWithLease"
	LeaseRevoke   TrafficRequestType = "leaseRevoke"
	CompareAndSet TrafficRequestType = "compareAndSet"
	Defragment    TrafficRequestType = "defragment"
)

type Traffic interface {
	Run(ctx context.Context, clientId int, c *recordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage)
}

type readWriteSingleKey struct {
	keyCount int
	writes   []requestChance
	leaseTTL int64
}

type requestChance struct {
	operation TrafficRequestType
	chance    int
}

func (t readWriteSingleKey) Run(ctx context.Context, clientId int, c *recordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage) {

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		key := fmt.Sprintf("%d", rand.Int()%t.keyCount)
		// Execute one read per one write to avoid operation history include too many failed writes when etcd is down.
		resp, err := t.Read(ctx, c, limiter, key)
		if err != nil {
			continue
		}
		// Provide each write with unique id to make it easier to validate operation history.
		t.Write(ctx, c, limiter, key, fmt.Sprintf("%d", ids.RequestId()), lm, clientId, resp)
	}
}

func (t readWriteSingleKey) Read(ctx context.Context, c *recordingClient, limiter *rate.Limiter, key string) ([]*mvccpb.KeyValue, error) {
	getCtx, cancel := context.WithTimeout(ctx, RequestTimeout)
	resp, err := c.Get(getCtx, key)
	cancel()
	if err == nil {
		limiter.Wait(ctx)
	}
	return resp, err
}

func (t readWriteSingleKey) Write(ctx context.Context, c *recordingClient, limiter *rate.Limiter, key string, newValue string, lm identity.LeaseIdStorage, cid int, lastValues []*mvccpb.KeyValue) error {
	writeCtx, cancel := context.WithTimeout(ctx, RequestTimeout)

	var err error
	switch t.pickWriteRequest() {
	case Put:
		err = c.Put(writeCtx, key, newValue)
	case Delete:
		err = c.Delete(writeCtx, key)
	case CompareAndSet:
		var expectValue string
		if len(lastValues) != 0 {
			expectValue = string(lastValues[0].Value)
		}
		err = c.CompareAndSet(writeCtx, key, expectValue, newValue)
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
			err = c.PutWithLease(putCtx, key, newValue, leaseId)
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
		panic("invalid operation")
	}
	cancel()
	if err == nil {
		limiter.Wait(ctx)
	}
	return err
}

func (t readWriteSingleKey) pickWriteRequest() TrafficRequestType {
	sum := 0
	for _, op := range t.writes {
		sum += op.chance
	}
	roll := rand.Int() % sum
	for _, op := range t.writes {
		if roll < op.chance {
			return op.operation
		}
		roll -= op.chance
	}
	panic("unexpected")
}
