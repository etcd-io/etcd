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
	"strings"
	"testing"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestV2CurlNoTLS(t *testing.T)      { testCurlPutGet(t, e2e.NewConfigNoTLS()) }
func TestV2CurlAutoTLS(t *testing.T)    { testCurlPutGet(t, e2e.NewConfigAutoTLS()) }
func TestV2CurlAllTLS(t *testing.T)     { testCurlPutGet(t, e2e.NewConfigTLS()) }
func TestV2CurlPeerTLS(t *testing.T)    { testCurlPutGet(t, e2e.NewConfigPeerTLS()) }
func TestV2CurlClientTLS(t *testing.T)  { testCurlPutGet(t, e2e.NewConfigClientTLS()) }
func TestV2CurlClientBoth(t *testing.T) { testCurlPutGet(t, e2e.NewConfigClientBoth()) }
func testCurlPutGet(t *testing.T, cfg *e2e.EtcdProcessClusterConfig) {
	BeforeTestV2(t)

	// test doesn't use quorum gets, so ensure there are no followers to avoid
	// stale reads that will break the test
	cfg = e2e.ConfigStandalone(*cfg)

	cfg.EnableV2 = true
	epc, err := e2e.NewEtcdProcessCluster(t, cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", err)
		}
	}()

	var (
		expectPut = `{"action":"set","node":{"key":"/foo","value":"bar","`
		expectGet = `{"action":"get","node":{"key":"/foo","value":"bar","`
	)
	if err := e2e.CURLPut(epc, e2e.CURLReq{Endpoint: "/v2/keys/foo", Value: "bar", Expected: expectPut}); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}
	if err := e2e.CURLGet(epc, e2e.CURLReq{Endpoint: "/v2/keys/foo", Expected: expectGet}); err != nil {
		t.Fatalf("failed get with curl (%v)", err)
	}
	if cfg.ClientTLS == e2e.ClientTLSAndNonTLS {
		if err := e2e.CURLGet(epc, e2e.CURLReq{Endpoint: "/v2/keys/foo", Expected: expectGet, IsTLS: true}); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	}
}

func TestV2CurlIssue5182(t *testing.T) {
	BeforeTestV2(t)

	copied := e2e.NewConfigNoTLS()
	copied.EnableV2 = true
	epc := setupEtcdctlTest(t, copied, false)
	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", err)
		}
	}()

	expectPut := `{"action":"set","node":{"key":"/foo","value":"bar","`
	if err := e2e.CURLPut(epc, e2e.CURLReq{Endpoint: "/v2/keys/foo", Value: "bar", Expected: expectPut}); err != nil {
		t.Fatal(err)
	}

	expectUserAdd := `{"user":"foo","roles":null}`
	if err := e2e.CURLPut(epc, e2e.CURLReq{Endpoint: "/v2/auth/users/foo", Value: `{"user":"foo", "password":"pass"}`, Expected: expectUserAdd}); err != nil {
		t.Fatal(err)
	}
	expectRoleAdd := `{"role":"foo","permissions":{"kv":{"read":["/foo/*"],"write":null}}`
	if err := e2e.CURLPut(epc, e2e.CURLReq{Endpoint: "/v2/auth/roles/foo", Value: `{"role":"foo", "permissions": {"kv": {"read": ["/foo/*"]}}}`, Expected: expectRoleAdd}); err != nil {
		t.Fatal(err)
	}
	expectUserUpdate := `{"user":"foo","roles":["foo"]}`
	if err := e2e.CURLPut(epc, e2e.CURLReq{Endpoint: "/v2/auth/users/foo", Value: `{"user": "foo", "grant": ["foo"]}`, Expected: expectUserUpdate}); err != nil {
		t.Fatal(err)
	}

	if err := etcdctlUserAdd(epc, "root", "a"); err != nil {
		t.Fatal(err)
	}
	if err := etcdctlAuthEnable(epc); err != nil {
		t.Fatal(err)
	}

	if err := e2e.CURLGet(epc, e2e.CURLReq{Endpoint: "/v2/keys/foo/", Username: "root", Password: "a", Expected: "bar"}); err != nil {
		t.Fatal(err)
	}
	if err := e2e.CURLGet(epc, e2e.CURLReq{Endpoint: "/v2/keys/foo/", Username: "foo", Password: "pass", Expected: "bar"}); err != nil {
		t.Fatal(err)
	}
	if err := e2e.CURLGet(epc, e2e.CURLReq{Endpoint: "/v2/keys/foo/", Username: "foo", Password: "", Expected: "bar"}); err != nil {
		if !strings.Contains(err.Error(), `The request requires user authentication`) {
			t.Fatalf("expected 'The request requires user authentication' error, got %v", err)
		}
	} else {
		t.Fatalf("expected 'The request requires user authentication' error")
	}
}
