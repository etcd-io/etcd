// Copyright 2024 The etcd Authors
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

package e2e

import "sync"

// uniquePorts is a global instance of testPorts.
var uniquePorts *testPorts

func init() {
	uniquePorts = newTestPorts(11000, 19000)
}

// testPorts is used to allocate listen ports for etcd instance in tests
// in a safe way for concurrent use (i.e. running tests in parallel).
type testPorts struct {
	mux    sync.Mutex
	unused map[int]bool
}

// newTestPorts keeps track of unused ports in the specified range.
func newTestPorts(start, end int) *testPorts {
	m := make(map[int]bool, end-start)
	for i := start; i < end; i++ {
		m[i] = true
	}
	return &testPorts{unused: m}
}

// Alloc allocates a new port or panics if none is available.
func (pa *testPorts) Alloc() int {
	pa.mux.Lock()
	defer pa.mux.Unlock()
	for port := range pa.unused {
		delete(pa.unused, port)
		return port
	}
	panic("all ports are used")
}

// Free makes port available for allocation through Alloc.
func (pa *testPorts) Free(port int) {
	pa.mux.Lock()
	defer pa.mux.Unlock()
	pa.unused[port] = true
}
