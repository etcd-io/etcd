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

package concurrency_test

import (
	"os"
	"testing"

	"go.etcd.io/etcd/pkg/v3/testutil"
	"go.etcd.io/etcd/tests/v3/integration"
)

var lazyCluster = integration.NewLazyCluster()

func exampleEndpoints() []string { return lazyCluster.EndpointsV3() }

func forUnitTestsRunInMockedContext(mocking func(), example func()) {
	// For integration tests runs in the provided environment
	example()
}

// TestMain sets up an etcd cluster if running the examples.
func TestMain(m *testing.M) {
	testutil.ExitInShortMode("Skipping: the tests require real cluster")

	v := m.Run()
	lazyCluster.Terminate()
	if v == 0 {
		testutil.MustCheckLeakedGoroutine()
	}
	os.Exit(v)
}
