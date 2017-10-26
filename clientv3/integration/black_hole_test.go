// Copyright 2017 The etcd Authors
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

// +build !cluster_proxy

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestBlackholePutWithoutKeealiveEnabled(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{
		Size:                 2,
		GRPCKeepAliveMinTime: 1 * time.Millisecond,
		SkipCreatingClient:   true},
	) // avoid too_many_pings

	defer clus.Terminate(t)

	ccfg := clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCAddr()},
		DialTimeout: 1 * time.Second,
	}
	cli, err := clientv3.New(ccfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// wait for ep[0] to be pinned
	waitPinReady(t, cli)

	cli.SetEndpoints(clus.Members[0].GRPCAddr(), clus.Members[1].GRPCAddr())
	clus.Members[0].Blackhole()

	// fail first due to blackhole, retry should succeed
	// when gRPC supports better retry on non-delivered request, the first put can succeed.
	ctx, c1 := context.WithTimeout(context.Background(), time.Second)
	defer c1()
	if _, err = cli.Put(ctx, "foo", "bar"); err != context.DeadlineExceeded {
		t.Fatalf("err = %v, want %v", err, context.DeadlineExceeded)
	}

	ctx, c2 := context.WithTimeout(context.Background(), time.Second)
	defer c2()
	if _, err = cli.Put(ctx, "foo", "bar"); err != nil {
		t.Errorf("put failed with error %v", err)
	}
}
