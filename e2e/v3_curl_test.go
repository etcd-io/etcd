// Copyright 2016 The etcd Authors
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
	"encoding/base64"
	"encoding/json"
	"path"
	"strconv"
	"testing"

	epb "github.com/coreos/etcd/etcdserver/api/v3election/v3electionpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/testutil"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

func TestV3CurlPutGetNoTLS(t *testing.T)     { testCurlPutGetGRPCGateway(t, &configNoTLS) }
func TestV3CurlPutGetAutoTLS(t *testing.T)   { testCurlPutGetGRPCGateway(t, &configAutoTLS) }
func TestV3CurlPutGetAllTLS(t *testing.T)    { testCurlPutGetGRPCGateway(t, &configTLS) }
func TestV3CurlPutGetPeerTLS(t *testing.T)   { testCurlPutGetGRPCGateway(t, &configPeerTLS) }
func TestV3CurlPutGetClientTLS(t *testing.T) { testCurlPutGetGRPCGateway(t, &configClientTLS) }
func testCurlPutGetGRPCGateway(t *testing.T, cfg *etcdProcessClusterConfig) {
	defer testutil.AfterTest(t)

	epc, err := newEtcdProcessCluster(cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if cerr := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", cerr)
		}
	}()

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
		t.Fatal(err)
	}
	rangeData, err := json.Marshal(&pb.RangeRequest{
		Key: key,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := cURLPost(epc, cURLReq{endpoint: "/v3alpha/kv/put", value: string(putData), expected: expectPut}); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}
	if err := cURLPost(epc, cURLReq{endpoint: "/v3alpha/kv/range", value: string(rangeData), expected: expectGet}); err != nil {
		t.Fatalf("failed get with curl (%v)", err)
	}

	if cfg.clientTLS == clientTLSAndNonTLS {
		if err := cURLPost(epc, cURLReq{endpoint: "/v3alpha/kv/range", value: string(rangeData), expected: expectGet, isTLS: true}); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	}
}

func TestV3CurlWatch(t *testing.T) {
	defer testutil.AfterTest(t)

	epc, err := newEtcdProcessCluster(&configNoTLS)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if cerr := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", cerr)
		}
	}()

	// store "bar" into "foo"
	putreq, err := json.Marshal(&pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	if err != nil {
		t.Fatal(err)
	}
	if err = cURLPost(epc, cURLReq{endpoint: "/v3alpha/kv/put", value: string(putreq), expected: "revision"}); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}
	// watch for first update to "foo"
	wcr := &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: 1}
	wreq, err := json.Marshal(wcr)
	if err != nil {
		t.Fatal(err)
	}
	// marshaling the grpc to json gives:
	// "{"RequestUnion":{"CreateRequest":{"key":"Zm9v","start_revision":1}}}"
	// but the gprc-gateway expects a different format..
	wstr := `{"create_request" : ` + string(wreq) + "}"
	// expects "bar", timeout after 2 seconds since stream waits forever
	if err = cURLPost(epc, cURLReq{endpoint: "/v3alpha/watch", value: wstr, expected: `"YmFy"`, timeout: 2}); err != nil {
		t.Fatal(err)
	}
}

func TestV3CurlTxn(t *testing.T) {
	defer testutil.AfterTest(t)
	epc, err := newEtcdProcessCluster(&configNoTLS)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if cerr := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", cerr)
		}
	}()

	txn := &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Key:         []byte("foo"),
				Result:      pb.Compare_EQUAL,
				Target:      pb.Compare_CREATE,
				TargetUnion: &pb.Compare_CreateRevision{0},
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
		t.Fatal(jerr)
	}
	expected := `"succeeded":true,"responses":[{"response_put":{"header":{"revision":"2"}}}]`
	if err = cURLPost(epc, cURLReq{endpoint: "/v3alpha/kv/txn", value: string(jsonDat), expected: expected}); err != nil {
		t.Fatalf("failed txn with curl (%v)", err)
	}

	// was crashing etcd server
	malformed := `{"compare":[{"result":0,"target":1,"key":"Zm9v","TargetUnion":null}],"success":[{"Request":{"RequestPut":{"key":"Zm9v","value":"YmFy"}}}]}`
	if err = cURLPost(epc, cURLReq{endpoint: "/v3alpha/kv/txn", value: malformed, expected: "error"}); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}
}

func TestV3CurlCampaignNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlCampaign, withApiPrefix(p), withCfg(configNoTLS))
	}
}

func testV3CurlCampaign(cx ctlCtx) {
	cdata, err := json.Marshal(&epb.CampaignRequest{
		Name:  []byte("/election-prefix"),
		Value: []byte("v1"),
	})
	if err != nil {
		cx.t.Fatal(err)
	}
	cargs := cURLPrefixArgs(cx.epc, "POST", cURLReq{
		endpoint: path.Join(cx.apiPrefix, "/election/campaign"),
		value:    string(cdata),
	})
	lines, err := spawnWithExpectLines(cargs, `"leader":{"name":"`)
	if err != nil {
		cx.t.Fatalf("failed post campaign request (%s) (%v)", cx.apiPrefix, err)
	}
	if len(lines) != 1 {
		cx.t.Fatalf("len(lines) expected 1, got %+v", lines)
	}

	var cresp campaignResponse
	if err = json.Unmarshal([]byte(lines[0]), &cresp); err != nil {
		cx.t.Fatalf("failed to unmarshal campaign response %v", err)
	}
	ndata, err := base64.StdEncoding.DecodeString(cresp.Leader.Name)
	if err != nil {
		cx.t.Fatalf("failed to decode leader key %v", err)
	}
	kdata, err := base64.StdEncoding.DecodeString(cresp.Leader.Key)
	if err != nil {
		cx.t.Fatalf("failed to decode leader key %v", err)
	}

	rev, _ := strconv.ParseInt(cresp.Leader.Rev, 10, 64)
	lease, _ := strconv.ParseInt(cresp.Leader.Lease, 10, 64)
	pdata, err := json.Marshal(&epb.ProclaimRequest{
		Leader: &epb.LeaderKey{
			Name:  ndata,
			Key:   kdata,
			Rev:   rev,
			Lease: lease,
		},
		Value: []byte("v2"),
	})
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = cURLPost(cx.epc, cURLReq{
		endpoint: path.Join(cx.apiPrefix, "/election/proclaim"),
		value:    string(pdata),
		expected: `"revision":`,
	}); err != nil {
		cx.t.Fatalf("failed post proclaim request (%s) (%v)", cx.apiPrefix, err)
	}
}

func TestV3CurlProclaimMissiongLeaderKeyNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlProclaimMissiongLeaderKey, withApiPrefix(p), withCfg(configNoTLS))
	}
}

func testV3CurlProclaimMissiongLeaderKey(cx ctlCtx) {
	pdata, err := json.Marshal(&epb.ProclaimRequest{Value: []byte("v2")})
	if err != nil {
		cx.t.Fatal(err)
	}
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = cURLPost(cx.epc, cURLReq{
		endpoint: path.Join(cx.apiPrefix, "/election/proclaim"),
		value:    string(pdata),
		expected: `{"error":"\"leader\" field must be provided","code":2}`,
	}); err != nil {
		cx.t.Fatalf("failed post proclaim request (%s) (%v)", cx.apiPrefix, err)
	}
}

func TestV3CurlResignMissiongLeaderKeyNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlResignMissiongLeaderKey, withApiPrefix(p), withCfg(configNoTLS))
	}
}

func testV3CurlResignMissiongLeaderKey(cx ctlCtx) {
	if err := cURLPost(cx.epc, cURLReq{
		endpoint: path.Join(cx.apiPrefix, "/election/resign"),
		value:    `{}`,
		expected: `{"error":"\"leader\" field must be provided","code":2}`,
	}); err != nil {
		cx.t.Fatalf("failed post resign request (%s) (%v)", cx.apiPrefix, err)
	}
}

// to manually decode; JSON marshals integer fields with
// string types, so can't unmarshal with epb.CampaignResponse
type campaignResponse struct {
	Leader struct {
		Name  string `json:"name,omitempty"`
		Key   string `json:"key,omitempty"`
		Rev   string `json:"rev,omitempty"`
		Lease string `json:"lease,omitempty"`
	} `json:"leader,omitempty"`
}
