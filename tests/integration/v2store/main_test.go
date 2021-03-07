// Copyright 2019 The etcd Authors
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

package v2store_test

import (
	"testing"

	"go.etcd.io/etcd/pkg/v3/testutil"
)

//var endpoints []string

func TestMain(m *testing.M) {
	//cfg := integration.ClusterConfig{Size: 1}
	//clus := integration.NewClusterV3(nil, &cfg)
	//endpoints = []string{clus.Client(0).Endpoints()[0]}
	//	v := m.Run()
	//clus.Terminate(nil)
	//if err := testutil.CheckAfterTest(time.Second); err != nil {
	//	fmt.Fprintf(os.Stderr, "%v", err)
	//	os.Exit(1)
	//}
	testutil.MustTestMainWithLeakDetection(m)
	//if v == 0 && testutil.CheckLeakedGoroutine() {
	//	os.Exit(1)
	//}
	//os.Exit(v)
}
