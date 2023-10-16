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

package failpoint

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

type trigger interface {
	Trigger(ctx context.Context, t *testing.T, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error
	AvailabilityChecker
}

type triggerDefrag struct{}

func (t triggerDefrag) Trigger(ctx context.Context, _ *testing.T, member e2e.EtcdProcess, _ *e2e.EtcdProcessCluster) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()
	_, err = cc.Defragment(ctx, member.EndpointsGRPC()[0])
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

func (t triggerDefrag) Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess) bool {
	return true
}

type triggerCompact struct {
	multiBatchCompaction bool
}

func (t triggerCompact) Trigger(ctx context.Context, _ *testing.T, member e2e.EtcdProcess, clus *e2e.EtcdProcessCluster) error {
	cc, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}
	defer cc.Close()

	var rev int64
	for {
		resp, gerr := cc.Get(ctx, "/")
		if gerr != nil {
			return gerr
		}

		rev = resp.Header.Revision
		if !t.multiBatchCompaction || rev > int64(clus.Cfg.ServerConfig.ExperimentalCompactionBatchLimit) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	_, err = cc.Compact(ctx, rev)
	if err != nil && !strings.Contains(err.Error(), "error reading from server: EOF") {
		return err
	}
	return nil
}

func (t triggerCompact) Available(e2e.EtcdProcessClusterConfig, e2e.EtcdProcess) bool {
	return true
}
