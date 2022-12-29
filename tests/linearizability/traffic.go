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
)

var (
	// DefaultLeaseTTL is set such that the lease does not expire on server side during the test. The test will exercise lease expiry using explicit lease revoke.
	DefaultLeaseTTL int64   = 7200
	DefaultKey              = "key"
	DefaultTraffic  Traffic = readWriteSingleKey{key: DefaultKey, leaseTTL: DefaultLeaseTTL, writes: []opChance{{operation: Put, chance: 80}, {operation: PutWithLease, chance: 5}, {operation: Delete, chance: 5}, {operation: LeaseRevoke, chance: 5}, {operation: Txn, chance: 5}}}
)

type Traffic interface {
	Run(ctx context.Context, c *recordingClient, limiter *rate.Limiter, ids idProvider)
}

type readWriteSingleKey struct {
	key      string
	leaseId  int64
	leaseTTL int64
	writes   []opChance
}

type opChance struct {
	operation Operation
	chance    int
}

func (t readWriteSingleKey) Run(ctx context.Context, c *recordingClient, limiter *rate.Limiter, ids idProvider) {

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Execute one read per one write to avoid operation history include too many failed writes when etcd is down.
		resp, err := t.Read(ctx, c, limiter)
		if err != nil {
			continue
		}
		// Provide each write with unique id to make it easier to validate operation history.
		t.Write(ctx, c, limiter, ids.RequestId(), resp)
	}
}

func (t readWriteSingleKey) Read(ctx context.Context, c *recordingClient, limiter *rate.Limiter) ([]*mvccpb.KeyValue, error) {
	getCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	resp, err := c.Get(getCtx, t.key)
	cancel()
	if err == nil {
		limiter.Wait(ctx)
	}
	return resp, err
}

func (t readWriteSingleKey) Write(ctx context.Context, c *recordingClient, limiter *rate.Limiter, id int, kvs []*mvccpb.KeyValue) error {
	putCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)

	var err error
	switch t.pickWriteOperation() {
	case Put:
		err = c.Put(putCtx, t.key, fmt.Sprintf("%d", id))
	case PutWithLease:
		if t.leaseId == 0 {
			t.leaseId, err = c.LeaseGrant(context.TODO(), t.leaseTTL)
		}
		err = c.PutWithLease(putCtx, t.key, fmt.Sprintf("%d", id), t.leaseId)
	case Delete:
		err = c.Delete(putCtx, t.key)
	case Txn:
		var value string
		if len(kvs) != 0 {
			value = string(kvs[0].Value)
		}
		err = c.Txn(putCtx, t.key, value, fmt.Sprintf("%d", id))
	case LeaseRevoke:
		err = c.LeaseRevoke(putCtx, t.leaseId)
		if err == nil {
			t.leaseId = 0
		}
	default:
		panic("invalid operation")
	}
	cancel()
	if err == nil {
		limiter.Wait(ctx)
	}
	return err
}

func (t readWriteSingleKey) pickWriteOperation() Operation {
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
