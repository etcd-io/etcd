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

package client_test

import (
	"net/http"
	"os"
	"testing"

	"go.etcd.io/etcd/tests/v3/integration"
	"go.etcd.io/etcd/v3/pkg/testutil"
)

var lazyCluster = integration.NewLazyCluster()

func exampleEndpoints() []string        { return lazyCluster.EndpointsV2() }
func exampleTransport() *http.Transport { return lazyCluster.Transport() }

func forUnitTestsRunInMockedContext(mocking func(), example func()) {
	// For integration tests runs in the provided environment
	example()
}

// TestMain sets up an etcd cluster if running the examples.
func TestMain(m *testing.M) {
	v := m.Run()
	lazyCluster.Terminate()
	if v == 0 {
		testutil.MustCheckLeakedGoroutine()
	}
	os.Exit(v)
}
