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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	epb "go.etcd.io/etcd/server/v3/etcdserver/api/v3election/v3electionpb"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var apiPrefix = []string{"/v3"}

func TestV3CurlWatch(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlWatch, withApiPrefix(p))
	}
}

func TestV3CurlAuth(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlAuth, withApiPrefix(p))
	}
}
func TestV3CurlAuthClientTLSCertAuth(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlAuth, withApiPrefix(p), withCfg(*e2e.NewConfigClientTLSCertAuthWithNoCN()))
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

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/put"), Value: string(putreq), Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
		cx.t.Fatalf("failed testV3CurlWatch put with curl using prefix (%s) (%v)", p, err)
	}
	// expects "bar", timeout after 2 seconds since stream waits forever
	err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/watch"), Value: wstr, Expected: expect.ExpectedResponse{Value: `"YmFy"`}, Timeout: 2})
	require.ErrorContains(cx.t, err, "unexpected exit code")
}

func testV3CurlAuth(cx ctlCtx) {
	p := cx.apiPrefix
	usernames := []string{"root", "nonroot", "nooption"}
	pwds := []string{"toor", "pass", "pass"}
	options := []*authpb.UserAddOptions{{NoPassword: false}, {NoPassword: false}, nil}

	// create users
	for i := 0; i < len(usernames); i++ {
		user, err := json.Marshal(&pb.AuthUserAddRequest{Name: usernames[i], Password: pwds[i], Options: options[i]})
		testutil.AssertNil(cx.t, err)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/auth/user/add"), Value: string(user), Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
			cx.t.Fatalf("failed testV3CurlAuth add user %v with curl (%v)", usernames[i], err)
		}
	}

	// create root role
	rolereq, err := json.Marshal(&pb.AuthRoleAddRequest{Name: "root"})
	testutil.AssertNil(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/auth/role/add"), Value: string(rolereq), Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth create role with curl using prefix (%s) (%v)", p, err)
	}

	//grant root role
	for i := 0; i < len(usernames); i++ {
		grantroleroot, err := json.Marshal(&pb.AuthUserGrantRoleRequest{User: usernames[i], Role: "root"})
		testutil.AssertNil(cx.t, err)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/auth/user/grant"), Value: string(grantroleroot), Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
			cx.t.Fatalf("failed testV3CurlAuth grant role with curl using prefix (%s) (%v)", p, err)
		}
	}

	// enable auth
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/auth/enable"), Value: "{}", Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
		cx.t.Fatalf("failed testV3CurlAuth enable auth with curl using prefix (%s) (%v)", p, err)
	}

	for i := 0; i < len(usernames); i++ {
		// put "bar[i]" into "foo[i]"
		putreq, err := json.Marshal(&pb.PutRequest{Key: []byte(fmt.Sprintf("foo%d", i)), Value: []byte(fmt.Sprintf("bar%d", i))})
		testutil.AssertNil(cx.t, err)

		// fail put no auth
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/put"), Value: string(putreq), Expected: expect.ExpectedResponse{Value: "error"}}); err != nil {
			cx.t.Fatalf("failed testV3CurlAuth no auth put with curl using prefix (%s) (%v)", p, err)
		}

		// auth request
		authreq, err := json.Marshal(&pb.AuthenticateRequest{Name: usernames[i], Password: pwds[i]})
		testutil.AssertNil(cx.t, err)

		var (
			authHeader string
			cmdArgs    []string
			lineFunc   = func(txt string) bool { return true }
		)

		cmdArgs = e2e.CURLPrefixArgsCluster(cx.epc.Cfg, cx.epc.Procs[rand.Intn(cx.epc.Cfg.ClusterSize)], "POST", e2e.CURLReq{Endpoint: path.Join(p, "/auth/authenticate"), Value: string(authreq)})
		proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
		testutil.AssertNil(cx.t, err)
		defer proc.Close()

		cURLRes, err := proc.ExpectFunc(context.Background(), lineFunc)
		testutil.AssertNil(cx.t, err)

		authRes := make(map[string]interface{})
		testutil.AssertNil(cx.t, json.Unmarshal([]byte(cURLRes), &authRes))

		token, ok := authRes[rpctypes.TokenFieldNameGRPC].(string)
		if !ok {
			cx.t.Fatalf("failed invalid token in authenticate response with curl using user (%v)", usernames[i])
		}

		authHeader = "Authorization: " + token

		// put with auth
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, "/kv/put"), Value: string(putreq), Header: authHeader, Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
			cx.t.Fatalf("failed testV3CurlAuth auth put with curl using prefix (%s) and user (%v) (%v)", p, usernames[i], err)
		}
	}
}

func TestV3CurlCampaignNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlCampaign, withApiPrefix(p), withCfg(*e2e.NewConfigNoTLS()))
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
	cargs := e2e.CURLPrefixArgsCluster(cx.epc.Cfg, cx.epc.Procs[rand.Intn(cx.epc.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/election/campaign"),
		Value:    string(cdata),
	})
	lines, err := e2e.SpawnWithExpectLines(context.TODO(), cargs, cx.envMap, expect.ExpectedResponse{Value: `"leader":{"name":"`})
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
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/election/proclaim"),
		Value:    string(pdata),
		Expected: expect.ExpectedResponse{Value: `"revision":`},
	}); err != nil {
		cx.t.Fatalf("failed post proclaim request (%s) (%v)", cx.apiPrefix, err)
	}
}

func TestV3CurlProclaimMissiongLeaderKeyNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlProclaimMissiongLeaderKey, withApiPrefix(p), withCfg(*e2e.NewConfigNoTLS()))
	}
}

func testV3CurlProclaimMissiongLeaderKey(cx ctlCtx) {
	pdata, err := json.Marshal(&epb.ProclaimRequest{Value: []byte("v2")})
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/election/proclaim"),
		Value:    string(pdata),
		Expected: expect.ExpectedResponse{Value: `{"error":"\"leader\" field must be provided","code":2,"message":"\"leader\" field must be provided"}`},
	}); err != nil {
		cx.t.Fatalf("failed post proclaim request (%s) (%v)", cx.apiPrefix, err)
	}
}

func TestV3CurlResignMissiongLeaderKeyNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlResignMissiongLeaderKey, withApiPrefix(p), withCfg(*e2e.NewConfigNoTLS()))
	}
}

func testV3CurlResignMissiongLeaderKey(cx ctlCtx) {
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/election/resign"),
		Value:    `{}`,
		Expected: expect.ExpectedResponse{Value: `{"error":"\"leader\" field must be provided","code":2,"message":"\"leader\" field must be provided"}`},
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

func CURLWithExpected(cx ctlCtx, tests []v3cURLTest) error {
	p := cx.apiPrefix
	for _, t := range tests {
		value := fmt.Sprintf("%v", t.value)
		if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: path.Join(p, t.endpoint), Value: value, Expected: expect.ExpectedResponse{Value: t.expected}}); err != nil {
			return fmt.Errorf("prefix (%s) endpoint (%s): error (%v), wanted %v", p, t.endpoint, err, t.expected)
		}
	}
	return nil
}
