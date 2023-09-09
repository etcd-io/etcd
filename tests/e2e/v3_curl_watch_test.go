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
	"testing"

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
	if err != nil {
		cx.t.Fatal(err)
	}
	// watch for first update to "foo"
	wcr := &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: 1}
	wreq, err := json.Marshal(wcr)
	if err != nil {
		cx.t.Fatal(err)
	}
	// marshaling the grpc to json gives:
	// "{"RequestUnion":{"CreateRequest":{"key":"Zm9v","start_revision":1}}}"
	// but the gprc-gateway expects a different format..
	wstr := `{"create_request" : ` + string(wreq) + "}"

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: "/v3/kv/put", Value: string(putreq), Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
		cx.t.Fatalf("failed testCurlV3Watch put with curl (%v)", err)
	}
	// expects "bar", timeout after 2 seconds since stream waits forever
	err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: "/v3/watch", Value: wstr, Expected: expect.ExpectedResponse{Value: `"YmFy"`}, Timeout: 2})
	require.ErrorContains(cx.t, err, "unexpected exit code")
}
