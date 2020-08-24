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

package examples_test

import (
	"fmt"
	"go.etcd.io/etcd/c/v3/clientv3"
	"google.golang.org/grpc/grpclog"
	"os"
	"testing"
	"time"

	"go.etcd.io/etcd/c/v3/pkg/testutil"
	"go.etcd.io/etcd/integration/v3"
)

var (
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
	endpoints      = []string{"localhost:2379", "localhost:22379", "localhost:32379"}
)

// TestMain sets up an etcd cluster if running the examples.
func TestMain(m *testing.M) {

	// Redirecting outputs to Stderr, such that they not interleave with examples outputs.
	// Setting it once and before running any of the test such that it not data-races
	// between HTTP servers running in different tests.
	clientv3.SetLogger(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))

	cfg := integration.ClusterConfig{Size: 3}
	clus := integration.NewClusterV3(nil, &cfg)
	endpoints = make([]string, 3)
	for i := range endpoints {
		endpoints[i] = clus.Client(i).Endpoints()[0]
	}
	v := m.Run()
	clus.Terminate(nil)
	if err := testutil.CheckAfterTest(time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}

	if v == 0 && testutil.CheckLeakedGoroutine() {
		os.Exit(1)
	}
	os.Exit(v)
}
