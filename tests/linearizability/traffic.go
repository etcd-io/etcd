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

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/time/rate"
)

var (
	DefaultTraffic Traffic = readWriteSingleKey{keyCount: 5, writes: []opChance{{operation: Put, chance: 60}, {operation: Delete, chance: 20}, {operation: Txn, chance: 20}}}
)

type Traffic interface {
	Run(ctx context.Context, c *recordingClient, limiter *rate.Limiter, ids idProvider)
}

type readWriteSingleKey struct {
	keyCount int
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
		key := fmt.Sprintf("%d", rand.Int()%t.keyCount)
		// Execute one read per one write to avoid operation history include too many failed writes when etcd is down.
		resp, err := t.Read(ctx, c, limiter, key)
		if err != nil {
			continue
		}
		// Provide each write with unique id to make it easier to validate operation history.
		t.Write(ctx, c, limiter, key, fmt.Sprintf("%d", ids.RequestId()), resp)
	}
}

func (t readWriteSingleKey) Read(ctx context.Context, c *recordingClient, limiter *rate.Limiter, key string) ([]*mvccpb.KeyValue, error) {
	getCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	resp, err := c.Get(getCtx, key)
	cancel()
	if err == nil {
		limiter.Wait(ctx)
	}
	return resp, err
}

func (t readWriteSingleKey) Write(ctx context.Context, c *recordingClient, limiter *rate.Limiter, key string, newValue string, lastValues []*mvccpb.KeyValue) error {
	putCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)

	var err error
	switch t.pickWriteOperation() {
	case Put:
		err = c.Put(putCtx, key, newValue)
	case Delete:
		err = c.Delete(putCtx, key)
	case Txn:
		var cmp clientv3.Cmp
		if len(lastValues) == 0 {
			cmp = clientv3.Compare(clientv3.CreateRevision(key), "=", 0)
		} else {
			cmp = clientv3.Compare(clientv3.Value(key), "=", string(lastValues[0].Value))
		}
		err = c.Txn(putCtx, []clientv3.Cmp{cmp}, []clientv3.Op{clientv3.OpPut(key, newValue)})
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
