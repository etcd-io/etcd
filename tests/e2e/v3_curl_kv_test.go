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
	"path"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

func TestV3CurlPutGetNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(*e2e.NewConfigNoTLS()))
	}
}
func TestV3CurlPutGetAutoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(*e2e.NewConfigAutoTLS()))
	}
}
func TestV3CurlPutGetAllTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(*e2e.NewConfigTLS()))
	}
}
func TestV3CurlPutGetPeerTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(*e2e.NewConfigPeerTLS()))
	}
}
func TestV3CurlPutGetClientTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(*e2e.NewConfigClientTLS()))
	}
}

func TestV3CurlTxn(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlTxn, withApiPrefix(p))
	}
}

func testV3CurlPutGet(cx ctlCtx) {
	var (
		key   = []byte("foo")
		value = []byte("bar") // this will be automatically base64-encoded by Go

		expectPut = `"revision":"`
		expectGet = `"value":"`
	)
	putData, err := json.Marshal(&pb.PutRequest{
		Key:   key,
		Value: value,
	})
	if err != nil {
		cx.t.Fatal(err)
	}
	rangeData, err := json.Marshal(&pb.RangeRequest{
		Key: key,
	})
	if err != nil {
		cx.t.Fatal(err)
	}

	p := cx.apiPrefix

	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/put"), Value: string(putData), Expected: expect.ExpectedResponse{Value: expectPut}}); err != nil {
		cx.t.Fatalf("failed testV3CurlPutGet put with curl using prefix (%s) (%v)", p, err)
	}
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/range"), Value: string(rangeData), Expected: expect.ExpectedResponse{Value: expectGet}}); err != nil {
		cx.t.Fatalf("failed testV3CurlPutGet get with curl using prefix (%s) (%v)", p, err)
	}
	if cx.cfg.Client.ConnectionType == e2e.ClientTLSAndNonTLS {
		if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/range"), Value: string(rangeData), Expected: expect.ExpectedResponse{Value: expectGet}, IsTLS: true}); err != nil {
			cx.t.Fatalf("failed testV3CurlPutGet get with curl using prefix (%s) (%v)", p, err)
		}
	}
}

func testV3CurlTxn(cx ctlCtx) {
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
	m := &runtime.JSONPb{}
	jsonDat, jerr := m.Marshal(txn)
	if jerr != nil {
		cx.t.Fatal(jerr)
	}
	expected := `"succeeded":true,"responses":[{"response_put":{"header":{"revision":"2"}}}]`
	p := cx.apiPrefix
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/txn"), Value: string(jsonDat), Expected: expect.ExpectedResponse{Value: expected}}); err != nil {
		cx.t.Fatalf("failed testV3CurlTxn txn with curl using prefix (%s) (%v)", p, err)
	}

	// was crashing etcd server
	malformed := `{"compare":[{"result":0,"target":1,"key":"Zm9v","TargetUnion":null}],"success":[{"Request":{"RequestPut":{"key":"Zm9v","value":"YmFy"}}}]}`
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/txn"), Value: malformed, Expected: expect.ExpectedResponse{Value: "error"}}); err != nil {
		cx.t.Fatalf("failed testV3CurlTxn put with curl using prefix (%s) (%v)", p, err)
	}
}
