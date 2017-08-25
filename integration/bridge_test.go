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

package integration

import (
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/testutil"

	"golang.org/x/net/context"
)

func TestBlackhole(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	clus.Members[0].Blackhole()
	time.Sleep(time.Second)

	clus.Members[0].Unblackhole()
	time.Sleep(time.Second)

	if _, err := clus.Client(0).Put(context.Background(), "foo", "bar"); err != nil {
		t.Fatal(err)
	}
}
