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
	"context"
	"encoding/json"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3Watch(t *testing.T) {
	testCtl(t, testCurlV3Watch)
}

func testCurlV3Watch(cx ctlCtx) {
	// store "bar" into "foo"
	putreq, err := json.Marshal(&pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	require.NoError(cx.t, err)
	// watch for first update to "foo"
	wcr := &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: 1}
	wreq, err := json.Marshal(wcr)
	require.NoError(cx.t, err)
	// marshaling the grpc to json gives:
	// "{"RequestUnion":{"CreateRequest":{"key":"Zm9v","start_revision":1}}}"
	// but the gprc-gateway expects a different format..
	wstr := `{"create_request" : ` + string(wreq) + "}"

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: "/v3/kv/put", Value: string(putreq), Expected: expect.ExpectedResponse{Value: "revision"}}), "failed testCurlV3Watch put with curl")
	// expects "bar", timeout after 2 seconds since stream waits forever
	require.ErrorContains(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: "/v3/watch", Value: wstr, Expected: expect.ExpectedResponse{Value: `"YmFy"`}, Timeout: 2}), "unexpected exit code")
}

// TestCurlWatchIssue19509 tries to reproduce https://github.com/etcd-io/etcd/issues/19509
func TestCurlWatchIssue19509(t *testing.T) {
	e2e.BeforeTest(t)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(e2e.NewConfigClientTLS()), e2e.WithClusterSize(1))
	require.NoError(t, err)
	defer epc.Close()

	curlCmdAndArgs := e2e.CURLPrefixArgsCluster(epc.Cfg, epc.Procs[0], "POST", e2e.CURLReq{Endpoint: "/v3/watch", Timeout: 3})
	for i := 0; i < 10; i++ {
		curlProc, err := e2e.SpawnCmd(curlCmdAndArgs, nil)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)

		_ = curlProc.Signal(syscall.SIGKILL)
		_ = curlProc.Close()

		require.Truef(t, epc.Procs[0].IsRunning(), "etcdserver already exited after %d curl watch requests", i+1)
	}
}
