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

package e2e

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3lock/v3lockpb"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3LockOperations(t *testing.T) {
	testCtl(t, testCurlV3LockOperations, withCfg(*e2e.NewConfig(e2e.WithClusterSize(1))))
}

func testCurlV3LockOperations(cx ctlCtx) {
	// lock
	lockReq, err := json.Marshal(&v3lockpb.LockRequest{Name: []byte("lock1")})
	require.NoError(cx.t, err)

	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[0], "POST", e2e.CURLReq{
		Endpoint: "/v3/lock/lock",
		Value:    string(lockReq),
	})
	resp, err := runCommandAndReadJSONOutput(args)
	require.NoError(cx.t, err)
	key, ok := resp["key"]
	require.True(cx.t, ok)

	// unlock
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/lock/unlock",
		Value:    fmt.Sprintf(`{"key": "%v"}`, key),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3LockOperations failed to execute unlock")
}
