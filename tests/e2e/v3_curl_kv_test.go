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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	protov1 "github.com/golang/protobuf/proto" //nolint:staticcheck // TODO: remove for a supported version
	gw "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3KVBasicOperation(t *testing.T) {
	testCurlV3KV(t, testCurlV3KVBasicOperation)
}

func TestCurlV3KVTxn(t *testing.T) {
	testCurlV3KV(t, testCurlV3KVTxn)
}

func TestCurlV3KVCompact(t *testing.T) {
	testCurlV3KV(t, testCurlV3KVCompact)
}

func testCurlV3KV(t *testing.T, f func(ctlCtx)) {
	testCases := []struct {
		name string
		cfg  ctlOption
	}{
		{
			name: "noTLS",
			cfg:  withCfg(*e2e.NewConfigNoTLS()),
		},
		{
			name: "autoTLS",
			cfg:  withCfg(*e2e.NewConfigAutoTLS()),
		},
		{
			name: "allTLS",
			cfg:  withCfg(*e2e.NewConfigTLS()),
		},
		{
			name: "peerTLS",
			cfg:  withCfg(*e2e.NewConfigPeerTLS()),
		},
		{
			name: "clientTLS",
			cfg:  withCfg(*e2e.NewConfigClientTLS()),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			testCtl(t, f, tc.cfg)
		})
	}
}

func testCurlV3KVBasicOperation(cx ctlCtx) {
	var (
		key   = []byte("foo")
		value = []byte("bar") // this will be automatically base64-encoded by Go

		expectedPutResponse    = `"revision":"`
		expectedGetResponse    = `"value":"`
		expectedDeleteResponse = `"deleted":"1"`
	)
	putData, err := json.Marshal(&pb.PutRequest{
		Key:   key,
		Value: value,
	})
	require.NoError(cx.t, err)

	rangeData, err := json.Marshal(&pb.RangeRequest{
		Key: key,
	})
	require.NoError(cx.t, err)

	deleteData, err := json.Marshal(&pb.DeleteRangeRequest{
		Key: key,
	})
	require.NoError(cx.t, err)

	err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/kv/put",
		Value:    string(putData),
		Expected: expect.ExpectedResponse{Value: expectedPutResponse},
	})
	require.NoErrorf(cx.t, err, "testCurlV3KVBasicOperation put failed")

	err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/kv/range",
		Value:    string(rangeData),
		Expected: expect.ExpectedResponse{Value: expectedGetResponse},
	})
	require.NoErrorf(cx.t, err, "testCurlV3KVBasicOperation get failed")

	if cx.cfg.Client.ConnectionType == e2e.ClientTLSAndNonTLS {
		err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/kv/range",
			Value:    string(rangeData),
			Expected: expect.ExpectedResponse{Value: expectedGetResponse},
			IsTLS:    true,
		})
		require.NoErrorf(cx.t, err, "testCurlV3KVBasicOperation get failed")
	}

	err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/kv/deleterange",
		Value:    string(deleteData),
		Expected: expect.ExpectedResponse{Value: expectedDeleteResponse},
	})
	require.NoErrorf(cx.t, err, "testCurlV3KVBasicOperation delete failed")
}

func testCurlV3KVTxn(cx ctlCtx) {
	txn := &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Key:         []byte("foo"),
				Result:      pb.Compare_EQUAL,
				Target:      pb.Compare_CREATE,
				TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 0},
			},
		},
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   []byte("foo"),
						Value: []byte("bar"),
					},
				},
			},
		},
	}
	m := gw.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: false,
		},
	}
	jsonDat, jerr := m.Marshal(protov1.MessageV2(txn))
	require.NoError(cx.t, jerr)

	succeeded, responses := mustExecuteTxn(cx, string(jsonDat))
	require.True(cx.t, succeeded)
	require.Len(cx.t, responses, 1)
	putResponse := responses[0].(map[string]any)
	_, ok := putResponse["response_put"]
	require.True(cx.t, ok)

	// was crashing etcd server
	malformed := `{"compare":[{"result":0,"target":1,"key":"Zm9v","TargetUnion":null}],"success":[{"Request":{"RequestPut":{"key":"Zm9v","value":"YmFy"}}}]}`
	err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/kv/txn",
		Value:    malformed,
		Expected: expect.ExpectedResponse{Value: "etcdserver: key not found"},
	})
	require.NoErrorf(cx.t, err, "testCurlV3Txn with malformed request failed")
}

func mustExecuteTxn(cx ctlCtx, reqData string) (bool, []any) {
	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[0], "POST", e2e.CURLReq{
		Endpoint: "/v3/kv/txn",
		Value:    reqData,
	})
	resp, err := runCommandAndReadJSONOutput(args)
	require.NoError(cx.t, err)

	succeeded, ok := resp["succeeded"]
	require.True(cx.t, ok)

	responses, ok := resp["responses"]
	require.True(cx.t, ok)

	return succeeded.(bool), responses.([]any)
}

func testCurlV3KVCompact(cx ctlCtx) {
	compactRequest, err := json.Marshal(&pb.CompactionRequest{
		Revision: 10000,
	})
	require.NoError(cx.t, err)

	err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/kv/compaction",
		Value:    string(compactRequest),
		Expected: expect.ExpectedResponse{
			Value: `"message":"etcdserver: mvcc: required revision is a future revision"`,
		},
	})
	require.NoErrorf(cx.t, err, "testCurlV3KVCompact failed")
}

func TestCurlV3KVOversizedRequestRejected(t *testing.T) {
	e2e.BeforeTest(t)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	clus, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
	)
	require.NoError(t, err)
	defer clus.Close()

	endpoint := clus.Procs[0].EndpointsHTTP()[0] + "/v3/kv/range"
	client := &http.Client{}

	t.Run("normal request succeeds", func(t *testing.T) {
		smallKey := base64.StdEncoding.EncodeToString([]byte("foo"))
		reqData := []byte(fmt.Sprintf(`{"key":"%s"}`, smallKey))

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqData))
		require.NoError(t, reqErr)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("oversized request rejected", func(t *testing.T) {
		largeKey := base64.StdEncoding.EncodeToString(make([]byte, 2*1024*1024))
		reqData := []byte(fmt.Sprintf(`{"key":"%s"}`, largeKey))

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqData))
		require.NoError(t, reqErr)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		t.Logf("oversized request response: status=%d body=%s", resp.StatusCode, string(body))

		require.NotEqual(t, http.StatusOK, resp.StatusCode,
			"oversized request must not succeed")
		require.NotEqual(t, http.StatusTooManyRequests, resp.StatusCode,
			"oversized request must be rejected before full body decode (was 429 before fix)")
		require.True(t,
			resp.StatusCode == http.StatusRequestEntityTooLarge ||
				resp.StatusCode == http.StatusBadRequest,
			"expected 413 or 400, got %d", resp.StatusCode)
	})
}
