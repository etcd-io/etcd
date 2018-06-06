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
	"fmt"
	"path"
	"strconv"
	"testing"

	epb "github.com/coreos/etcd/etcdserver/api/v3election/v3electionpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/version"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

// TODO: remove /v3alpha tests in 3.4 release

func TestV3CurlPutGetNoTLSAlpha(t *testing.T) { testCurlPutGetGRPCGateway(t, &configNoTLS, "/v3alpha") }
func TestV3CurlPutGetNoTLSBeta(t *testing.T)  { testCurlPutGetGRPCGateway(t, &configNoTLS, "/v3beta") }
func TestV3CurlPutGetAutoTLSAlpha(t *testing.T) {
	testCurlPutGetGRPCGateway(t, &configAutoTLS, "/v3alpha")
}
func TestV3CurlPutGetAutoTLSBeta(t *testing.T) {
	testCurlPutGetGRPCGateway(t, &configAutoTLS, "/v3beta")
}
func TestV3CurlPutGetAllTLSAlpha(t *testing.T) { testCurlPutGetGRPCGateway(t, &configTLS, "/v3alpha") }
func TestV3CurlPutGetAllTLSBeta(t *testing.T)  { testCurlPutGetGRPCGateway(t, &configTLS, "/v3beta") }
func TestV3CurlPutGetPeerTLSAlpha(t *testing.T) {
	testCurlPutGetGRPCGateway(t, &configPeerTLS, "/v3alpha")
}
func TestV3CurlPutGetPeerTLSBeta(t *testing.T) {
	testCurlPutGetGRPCGateway(t, &configPeerTLS, "/v3beta")
}
func TestV3CurlPutGetClientTLSAlpha(t *testing.T) {
	testCurlPutGetGRPCGateway(t, &configClientTLS, "/v3alpha")
}
func TestV3CurlPutGetClientTLSBeta(t *testing.T) {
	testCurlPutGetGRPCGateway(t, &configClientTLS, "/v3beta")
}
func testCurlPutGetGRPCGateway(t *testing.T, cfg *etcdProcessClusterConfig, pathPrefix string) {
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

	if err := cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/put"), value: string(putData), expected: expectPut}); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}
	if err := cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/range"), value: string(rangeData), expected: expectGet}); err != nil {
		t.Fatalf("failed get with curl (%v)", err)
	}

	if cfg.clientTLS == clientTLSAndNonTLS {
		if err := cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/range"), value: string(rangeData), expected: expectGet, isTLS: true}); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	}
}

func TestV3CurlWatchAlpha(t *testing.T) { testV3CurlWatch(t, "/v3alpha") }
func TestV3CurlWatchBeta(t *testing.T)  { testV3CurlWatch(t, "/v3beta") }
func testV3CurlWatch(t *testing.T, pathPrefix string) {
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
	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/put"), value: string(putreq), expected: "revision"}); err != nil {
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
	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/watch"), value: wstr, expected: `"YmFy"`, timeout: 2}); err != nil {
		t.Fatal(err)
	}
}

func TestV3CurlTxnAlpha(t *testing.T) { testV3CurlTxn(t, "/v3alpha") }
func TestV3CurlTxnBeta(t *testing.T)  { testV3CurlTxn(t, "/v3beta") }
func testV3CurlTxn(t *testing.T, pathPrefix string) {
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
	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/txn"), value: string(jsonDat), expected: expected}); err != nil {
		t.Fatalf("failed txn with curl (%v)", err)
	}

	// was crashing etcd server
	malformed := `{"compare":[{"result":0,"target":1,"key":"Zm9v","TargetUnion":null}],"success":[{"Request":{"RequestPut":{"key":"Zm9v","value":"YmFy"}}}]}`
	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/txn"), value: malformed, expected: "error"}); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}
}

func TestV3CurlAuthAlpha(t *testing.T) { testV3CurlAuth(t, "/v3alpha") }
func TestV3CurlAuthBeta(t *testing.T)  { testV3CurlAuth(t, "/v3beta") }
func testV3CurlAuth(t *testing.T, pathPrefix string) {
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

	// create root user
	userreq, err := json.Marshal(&pb.AuthUserAddRequest{Name: string("root"), Password: string("toor")})
	testutil.AssertNil(t, err)

	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/auth/user/add"), value: string(userreq), expected: "revision"}); err != nil {
		t.Fatalf("failed add user with curl (%v)", err)
	}

	// create root role
	rolereq, err := json.Marshal(&pb.AuthRoleAddRequest{Name: string("root")})
	testutil.AssertNil(t, err)

	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/auth/role/add"), value: string(rolereq), expected: "revision"}); err != nil {
		t.Fatalf("failed create role with curl (%v)", err)
	}

	// grant root role
	grantrolereq, err := json.Marshal(&pb.AuthUserGrantRoleRequest{User: string("root"), Role: string("root")})
	testutil.AssertNil(t, err)

	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/auth/user/grant"), value: string(grantrolereq), expected: "revision"}); err != nil {
		t.Fatalf("failed grant role with curl (%v)", err)
	}

	// enable auth
	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/auth/enable"), value: string("{}"), expected: "revision"}); err != nil {
		t.Fatalf("failed enable auth with curl (%v)", err)
	}

	// put "bar" into "foo"
	putreq, err := json.Marshal(&pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	testutil.AssertNil(t, err)

	// fail put no auth
	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/put"), value: string(putreq), expected: "error"}); err != nil {
		t.Fatalf("failed no auth put with curl (%v)", err)
	}

	// auth request
	authreq, err := json.Marshal(&pb.AuthenticateRequest{Name: string("root"), Password: string("toor")})
	testutil.AssertNil(t, err)

	var (
		authHeader string
		cmdArgs    []string
		lineFunc   = func(txt string) bool { return true }
	)

	cmdArgs = cURLPrefixArgs(epc, "POST", cURLReq{endpoint: path.Join(pathPrefix, "/auth/authenticate"), value: string(authreq)})
	proc, err := spawnCmd(cmdArgs)
	testutil.AssertNil(t, err)

	cURLRes, err := proc.ExpectFunc(lineFunc)
	testutil.AssertNil(t, err)

	authRes := make(map[string]interface{})
	testutil.AssertNil(t, json.Unmarshal([]byte(cURLRes), &authRes))

	token, ok := authRes["token"].(string)
	if !ok {
		t.Fatalf("failed invalid token in authenticate response with curl")
	}

	authHeader = "Authorization : " + token

	// put with auth
	if err = cURLPost(epc, cURLReq{endpoint: path.Join(pathPrefix, "/kv/put"), value: string(putreq), header: authHeader, expected: "revision"}); err != nil {
		t.Fatalf("failed auth put with curl (%v)", err)
	}
}

func TestV3CurlCampaignAlpha(t *testing.T) { testV3CurlCampaign(t, "/v3alpha") }
func TestV3CurlCampaignBeta(t *testing.T)  { testV3CurlCampaign(t, "/v3beta") }
func testV3CurlCampaign(t *testing.T, pathPrefix string) {
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

	cdata, err := json.Marshal(&epb.CampaignRequest{
		Name:  []byte("/election-prefix"),
		Value: []byte("v1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	cargs := cURLPrefixArgs(epc, "POST", cURLReq{
		endpoint: path.Join(pathPrefix, "/election/campaign"),
		value:    string(cdata),
	})
	lines, err := spawnWithExpectLines(cargs, `"leader":{"name":"`)
	if err != nil {
		t.Fatalf("failed post campaign request (%s) (%v)", pathPrefix, err)
	}
	if len(lines) != 1 {
		t.Fatalf("len(lines) expected 1, got %+v", lines)
	}

	var cresp campaignResponse
	if err = json.Unmarshal([]byte(lines[0]), &cresp); err != nil {
		t.Fatalf("failed to unmarshal campaign response %v", err)
	}
	ndata, err := base64.StdEncoding.DecodeString(cresp.Leader.Name)
	if err != nil {
		t.Fatalf("failed to decode leader key %v", err)
	}
	kdata, err := base64.StdEncoding.DecodeString(cresp.Leader.Key)
	if err != nil {
		t.Fatalf("failed to decode leader key %v", err)
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
		t.Fatal(err)
	}
	if err = cURLPost(epc, cURLReq{
		endpoint: path.Join(pathPrefix, "/election/proclaim"),
		value:    string(pdata),
		expected: `"revision":`,
	}); err != nil {
		t.Fatalf("failed post proclaim request (%s) (%v)", pathPrefix, err)
	}
}

func TestV3CurlProclaimMissiongLeaderKeyNoTLS(t *testing.T) {
	testCtl(t, testV3CurlProclaimMissiongLeaderKey, withCfg(configNoTLS))
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
		endpoint: path.Join("/v3beta", "/election/proclaim"),
		value:    string(pdata),
		expected: `{"error":"\"leader\" field must be provided","code":2}`,
	}); err != nil {
		cx.t.Fatalf("failed post proclaim request (%s) (%v)", "/v3beta", err)
	}
}

func TestV3CurlResignMissiongLeaderKeyNoTLS(t *testing.T) {
	testCtl(t, testV3CurlResignMissiongLeaderKey, withCfg(configNoTLS))
}

func testV3CurlResignMissiongLeaderKey(cx ctlCtx) {
	if err := cURLPost(cx.epc, cURLReq{
		endpoint: path.Join("/v3beta", "/election/resign"),
		value:    `{}`,
		expected: `{"error":"\"leader\" field must be provided","code":2}`,
	}); err != nil {
		cx.t.Fatalf("failed post resign request (%s) (%v)", "/v3beta", err)
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

func TestV3CurlCipherSuitesValid(t *testing.T)    { testV3CurlCipherSuites(t, true) }
func TestV3CurlCipherSuitesMismatch(t *testing.T) { testV3CurlCipherSuites(t, false) }
func testV3CurlCipherSuites(t *testing.T, valid bool) {
	cc := configClientTLS
	cc.clusterSize = 1
	cc.cipherSuites = []string{
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
	}
	testFunc := cipherSuiteTestValid
	if !valid {
		testFunc = cipherSuiteTestMismatch
	}
	testCtl(t, testFunc, withCfg(cc))
}

func cipherSuiteTestValid(cx ctlCtx) {
	if err := cURLGet(cx.epc, cURLReq{
		endpoint:         "/metrics",
		expected:         fmt.Sprintf(`etcd_server_version{server_version="%s"} 1`, version.Version),
		metricsURLScheme: cx.cfg.metricsURLScheme,
		ciphers:          "ECDHE-RSA-AES128-GCM-SHA256", // TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}

func cipherSuiteTestMismatch(cx ctlCtx) {
	if err := cURLGet(cx.epc, cURLReq{
		endpoint:         "/metrics",
		expected:         "alert handshake failure",
		metricsURLScheme: cx.cfg.metricsURLScheme,
		ciphers:          "ECDHE-RSA-DES-CBC3-SHA", // TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
	}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}
