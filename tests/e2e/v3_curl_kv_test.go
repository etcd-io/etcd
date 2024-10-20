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

	protov1 "github.com/golang/protobuf/proto"
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
