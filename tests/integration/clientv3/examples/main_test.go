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

package clientv3test

import (
	"fmt"
	"go.etcd.io/etcd/tests/v3/integration"
	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/pkg/testutil"
	"google.golang.org/grpc/grpclog"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	dialTimeout    = 5 * time.Second
	requestTimeout = 10 * time.Second
	endpoints      = []string{"localhost:2379", "localhost:22379", "localhost:32379"}
)

// TestMain sets up an etcd cluster if running the examples.
func TestMain(m *testing.M) {
	useCluster, hasRunArg := false, false // default to running only Test*
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.run=") {
			exp := strings.Split(arg, "=")[1]
			useCluster = strings.Contains(exp, "Example")
			hasRunArg = true
			break
		}
	}
	if !hasRunArg {
		// force only running Test* if no args given to avoid leak false
		// positives from having a long-running cluster for the examples.
		os.Args = append(os.Args, "-test.run=Test")
	}

	var v int
	if useCluster {
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
		v = m.Run()
		clus.Terminate(nil)
		if err := testutil.CheckAfterTest(time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	} else {
		v = m.Run()
	}
	if v == 0 && testutil.CheckLeakedGoroutine() {
		os.Exit(1)
	}
	os.Exit(v)
}
