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
	"fmt"
	"math/rand"
	"testing"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestV3CurlAuth(t *testing.T) {
	testCtl(t, testV3CurlAuth)
}
func TestV3CurlAuthClientTLSCertAuth(t *testing.T) {
	testCtl(t, testV3CurlAuth, withCfg(*e2e.NewConfigClientTLSCertAuthWithNoCN()))
}

func testV3CurlAuth(cx ctlCtx) {
	usernames := []string{"root", "nonroot", "nooption"}
	pwds := []string{"toor", "pass", "pass"}
	options := []*authpb.UserAddOptions{{NoPassword: false}, {NoPassword: false}, nil}

	// create users
	for i := 0; i < len(usernames); i++ {
		user, err := json.Marshal(&pb.AuthUserAddRequest{Name: usernames[i], Password: pwds[i], Options: options[i]})
		testutil.AssertNil(cx.t, err)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/add",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testV3CurlAuth failed to add user %v (%v)", usernames[i], err)
		}
	}

	// create root role
	rolereq, err := json.Marshal(&pb.AuthRoleAddRequest{Name: "root"})
	testutil.AssertNil(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/add",
		Value:    string(rolereq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testV3CurlAuth failed to create role (%v)", err)
	}

	//grant root role
	for i := 0; i < len(usernames); i++ {
		grantroleroot, err := json.Marshal(&pb.AuthUserGrantRoleRequest{User: usernames[i], Role: "root"})
		testutil.AssertNil(cx.t, err)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/grant",
			Value:    string(grantroleroot),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testV3CurlAuth failed to grant role (%v)", err)
		}
	}

	// enable auth
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/enable",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testV3CurlAuth failed to enable auth (%v)", err)
	}

	for i := 0; i < len(usernames); i++ {
		// put "bar[i]" into "foo[i]"
		putreq, err := json.Marshal(&pb.PutRequest{Key: []byte(fmt.Sprintf("foo%d", i)), Value: []byte(fmt.Sprintf("bar%d", i))})
		testutil.AssertNil(cx.t, err)

		// fail put no auth
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/kv/put",
			Value:    string(putreq),
			Expected: expect.ExpectedResponse{Value: "error"},
		}); err != nil {
			cx.t.Fatalf("testV3CurlAuth failed to put without token (%v)", err)
		}

		// auth request
		authreq, err := json.Marshal(&pb.AuthenticateRequest{Name: usernames[i], Password: pwds[i]})
		testutil.AssertNil(cx.t, err)

		var (
			authHeader string
			cmdArgs    []string
			lineFunc   = func(txt string) bool { return true }
		)

		cmdArgs = e2e.CURLPrefixArgsCluster(cx.epc.Cfg, cx.epc.Procs[rand.Intn(cx.epc.Cfg.ClusterSize)], "POST", e2e.CURLReq{
			Endpoint: "/v3/auth/authenticate",
			Value:    string(authreq),
		})
		proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
		testutil.AssertNil(cx.t, err)
		defer proc.Close()

		cURLRes, err := proc.ExpectFunc(context.Background(), lineFunc)
		testutil.AssertNil(cx.t, err)

		authRes := make(map[string]interface{})
		testutil.AssertNil(cx.t, json.Unmarshal([]byte(cURLRes), &authRes))

		token, ok := authRes[rpctypes.TokenFieldNameGRPC].(string)
		if !ok {
			cx.t.Fatalf("failed invalid token in authenticate response using user (%v)", usernames[i])
		}

		authHeader = "Authorization: " + token
		// put with auth
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/kv/put",
			Value:    string(putreq),
			Header:   authHeader,
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testV3CurlAuth failed to auth put with user (%v) (%v)", usernames[i], err)
		}
	}
}
