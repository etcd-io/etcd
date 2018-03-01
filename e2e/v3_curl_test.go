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

// TODO: remove /v3beta tests in 3.5 release
var apiPrefix = []string{"/v3", "/v3beta"}

func TestV3CurlPutGetNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(configNoTLS))
	}
}
func TestV3CurlPutGetAutoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(configAutoTLS))
	}
}
func TestV3CurlPutGetAllTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(configTLS))
	}
}
func TestV3CurlPutGetPeerTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(configPeerTLS))
	}
}
func TestV3CurlPutGetClientTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlPutGet, withApiPrefix(p), withCfg(configClientTLS))
	}
}
func TestV3CurlWatch(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlWatch, withApiPrefix(p))
	}
}
func TestV3CurlTxn(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlTxn, withApiPrefix(p))
	}
}
func TestV3CurlAuth(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlAuth, withApiPrefix(p))
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

	if err := cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/put"), value: string(putData), expected: expectPut}); err != nil {
		cx.t.Fatalf("failed testV3CurlPutGet put with curl using prefix (%s) (%v)", p, err)
	}
	if err := cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/range"), value: string(rangeData), expected: expectGet}); err != nil {
		cx.t.Fatalf("failed testV3CurlPutGet get with curl using prefix (%s) (%v)", p, err)
	}
	if cx.cfg.clientTLS == clientTLSAndNonTLS {
		if err := cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/range"), value: string(rangeData), expected: expectGet, isTLS: true}); err != nil {
			cx.t.Fatalf("failed testV3CurlPutGet get with curl using prefix (%s) (%v)", p, err)
		}
	}
}

func testV3CurlWatch(cx ctlCtx) {
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
	p := cx.apiPrefix

	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/put"), value: string(putreq), expected: "revision"}); err != nil {
		cx.t.Fatalf("failed testV3CurlWatch put with curl using prefix (%s) (%v)", p, err)
	}
	// expects "bar", timeout after 2 seconds since stream waits forever
	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/watch"), value: wstr, expected: `"YmFy"`, timeout: 2}); err != nil {
		cx.t.Fatalf("failed testV3CurlWatch watch with curl using prefix (%s) (%v)", p, err)
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
	if err := cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/txn"), value: string(jsonDat), expected: expected}); err != nil {
		cx.t.Fatalf("failed testV3CurlTxn txn with curl using prefix (%s) (%v)", p, err)
	}

	// was crashing etcd server
	malformed := `{"compare":[{"result":0,"target":1,"key":"Zm9v","TargetUnion":null}],"success":[{"Request":{"RequestPut":{"key":"Zm9v","value":"YmFy"}}}]}`
	if err := cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/txn"), value: malformed, expected: "error"}); err != nil {
		cx.t.Fatalf("failed testV3CurlTxn put with curl using prefix (%s) (%v)", p, err)
	}

}

func testV3CurlAuth(cx ctlCtx) {
	// create root user
	userreq, err := json.Marshal(&pb.AuthUserAddRequest{Name: string("root"), Password: string("toor")})
	testutil.AssertNil(cx.t, err)

	p := cx.apiPrefix
	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/auth/user/add"), value: string(userreq), expected: "revision"}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth add user with curl (%v)", err)
	}

	// create root role
	rolereq, err := json.Marshal(&pb.AuthRoleAddRequest{Name: string("root")})
	testutil.AssertNil(cx.t, err)

	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/auth/role/add"), value: string(rolereq), expected: "revision"}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth create role with curl using prefix (%s) (%v)", p, err)
	}

	// grant root role
	grantrolereq, err := json.Marshal(&pb.AuthUserGrantRoleRequest{User: string("root"), Role: string("root")})
	testutil.AssertNil(cx.t, err)

	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/auth/user/grant"), value: string(grantrolereq), expected: "revision"}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth grant role with curl using prefix (%s) (%v)", p, err)
	}

	// enable auth
	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/auth/enable"), value: string("{}"), expected: "revision"}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth enable auth with curl using prefix (%s) (%v)", p, err)
	}

	// put "bar" into "foo"
	putreq, err := json.Marshal(&pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	testutil.AssertNil(cx.t, err)

	// fail put no auth
	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/put"), value: string(putreq), expected: "error"}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth no auth put with curl using prefix (%s) (%v)", p, err)
	}

	// auth request
	authreq, err := json.Marshal(&pb.AuthenticateRequest{Name: string("root"), Password: string("toor")})
	testutil.AssertNil(cx.t, err)

	var (
		authHeader string
		cmdArgs    []string
		lineFunc   = func(txt string) bool { return true }
	)

	cmdArgs = cURLPrefixArgs(cx.epc, "POST", cURLReq{endpoint: path.Join(p, "/auth/authenticate"), value: string(authreq)})
	proc, err := spawnCmd(cmdArgs)
	testutil.AssertNil(cx.t, err)

	cURLRes, err := proc.ExpectFunc(lineFunc)
	testutil.AssertNil(cx.t, err)

	authRes := make(map[string]interface{})
	testutil.AssertNil(cx.t, json.Unmarshal([]byte(cURLRes), &authRes))

	token, ok := authRes["token"].(string)
	if !ok {
		cx.t.Fatalf("failed invalid token in authenticate response with curl")
	}

	authHeader = "Authorization : " + token

	// put with auth
	if err = cURLPost(cx.epc, cURLReq{endpoint: path.Join(p, "/kv/put"), value: string(putreq), header: authHeader, expected: "revision"}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth auth put with curl using prefix (%s) (%v)", p, err)
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
