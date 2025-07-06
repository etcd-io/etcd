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
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
)

func exampleEndpoints() []string { return nil }

func forUnitTestsRunInMockedContext(mocking func(), example func()) {
	mocking()
	// TODO: Call 'example' when mocking() provides realistic mocking of transport.

	// The real testing logic of examples gets executed
	// as part of ./tests/integration/clientv3/integration/...
}

func TestMain(m *testing.M) {
	testutil.MustTestMainWithLeakDetection(m)
}
