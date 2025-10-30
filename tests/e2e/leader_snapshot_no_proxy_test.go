// Copyright 2016 The etcd Authors
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

//go:build !cluster_proxy

package e2e

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/failpoint"
)

func TestRecoverSnapshotBackend(t *testing.T) {
	e2e.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(3),
		e2e.WithKeepDataDir(true),
		e2e.WithPeerProxy(true),
		e2e.WithSnapshotCatchUpEntries(50),
		e2e.WithSnapshotCount(50),
		e2e.WithGoFailEnabled(true),
		e2e.WithIsPeerTLS(true),
	)
	require.NoError(t, err)

	defer epc.Close()

	blackholedMember := epc.Procs[0]
	otherMember := epc.Procs[1]

	wg := sync.WaitGroup{}

	trafficCtx, trafficCancel := context.WithCancel(ctx)
	c, err := clientv3.New(clientv3.Config{
		Endpoints:            otherMember.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	defer c.Close()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-trafficCtx.Done():
				return
			default:
			}
			putCtx, putCancel := context.WithTimeout(trafficCtx, 50*time.Millisecond)
			c.Put(putCtx, "a", "b")
			putCancel()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	err = blackholedMember.Failpoints().SetupHTTP(ctx, "applyBeforeOpenSnapshot", "panic")
	require.NoError(t, err)
	err = failpoint.Blackhole(ctx, t, blackholedMember, epc, true)
	require.NoError(t, err)
	err = blackholedMember.Wait(ctx)
	require.NoError(t, err)
	trafficCancel()
	wg.Wait()
	err = blackholedMember.Start(ctx)
	require.NoError(t, err)
	_, err = blackholedMember.Logs().ExpectWithContext(ctx, expect.ExpectedResponse{Value: "Recovering from snapshot backend"})
	require.NoError(t, err)
	err = blackholedMember.Etcdctl().Put(ctx, "a", "1", config.PutOptions{})
	assert.NoError(t, err)
}
