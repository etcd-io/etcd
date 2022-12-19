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
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/linearizability/identity"
	"go.etcd.io/etcd/tests/v3/linearizability/model"
)

var (
	startKey                       = "0"
	endKey                         = fmt.Sprintf("%d", math.MaxInt)
	DefaultLeaseTTL        int64   = 7200
	RequestTimeout                 = 40 * time.Millisecond
	DefaultTraffic         Traffic = readWriteSingleKey{keyCount: 4, leaseTTL: DefaultLeaseTTL, writes: []opChance{{operation: model.Put, chance: 50}, {operation: model.Delete, chance: 10}, {operation: model.PutWithLease, chance: 10}, {operation: model.LeaseRevoke, chance: 10}, {operation: model.Txn, chance: 20}}}
	DefaultTrafficWithAuth Traffic = readWriteSingleKey{authEnabled: true, keyCount: 4, leaseTTL: DefaultLeaseTTL, writes: []opChance{{operation: model.Put, chance: 50}, {operation: model.Delete, chance: 10}, {operation: model.PutWithLease, chance: 10}, {operation: model.LeaseRevoke, chance: 10}, {operation: model.Txn, chance: 20}}}
)

type Traffic interface {
	PreRun(ctx context.Context, c interfaces.Client, lg *zap.Logger) error
	Run(ctx context.Context, clientId int, c *recordingClient, limiter *rate.Limiter, ids identity.Provider, lm identity.LeaseIdStorage)
	AuthEnabled() bool
}

type readWriteSingleKey struct {
	keyCount int
	writes   []opChance
	leaseTTL int64

	authEnabled bool
}

type opChance struct {
	operation model.OperationType
	chance    int
}

func (t readWriteSingleKey) PreRun(ctx context.Context, c interfaces.Client, lg *zap.Logger) error {
	if t.AuthEnabled() {
		lg.Info("set up auth")
		return setupAuth(ctx, c)
	}
	return nil
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

func (t readWriteSingleKey) AuthEnabled() bool {
	return t.authEnabled
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
	switch t.pickWriteOperation() {
	case model.Put:
		err = c.Put(writeCtx, key, newValue)
	case model.Delete:
		err = c.Delete(writeCtx, key)
	case model.Txn:
		var expectValue string
		if len(lastValues) != 0 {
			expectValue = string(lastValues[0].Value)
		}
		err = c.Txn(writeCtx, key, expectValue, newValue)
	case model.PutWithLease:
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
	case model.LeaseRevoke:
		leaseId := lm.LeaseId(cid)
		if leaseId != 0 {
			err = c.LeaseRevoke(writeCtx, leaseId)
			//if LeaseRevoke has failed, do not remove the mapping.
			if err == nil {
				lm.RemoveLeaseId(cid)
			}
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

func (t readWriteSingleKey) pickWriteOperation() model.OperationType {
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

var (
	users = []struct {
		userName     string
		userPassword string
	}{
		{rootUserName, rootUserPassword},
		{testUserName, testUserPassword},
	}
)

const (
	rootUserName     = "root"
	rootRoleName     = "root"
	rootUserPassword = "123"
	testUserName     = "test-user"
	testRoleName     = "test-role"
	testUserPassword = "abc"
)

func setupAuth(ctx context.Context, c interfaces.Client) error {
	if _, err := c.UserAdd(ctx, rootUserName, rootUserPassword, config.UserAddOptions{}); err != nil {
		return err
	}
	if _, err := c.RoleAdd(ctx, rootRoleName); err != nil {
		return err
	}
	if _, err := c.UserGrantRole(ctx, rootUserName, rootRoleName); err != nil {
		return err
	}
	if err := c.AuthEnable(ctx); err != nil {
		return err
	}
	return nil
}
