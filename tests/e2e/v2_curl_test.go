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
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestV2CurlNoTLS(t *testing.T)      { testCurlPutGet(t, newConfigNoTLS()) }
func TestV2CurlAutoTLS(t *testing.T)    { testCurlPutGet(t, newConfigAutoTLS()) }
func TestV2CurlAllTLS(t *testing.T)     { testCurlPutGet(t, newConfigTLS()) }
func TestV2CurlPeerTLS(t *testing.T)    { testCurlPutGet(t, newConfigPeerTLS()) }
func TestV2CurlClientTLS(t *testing.T)  { testCurlPutGet(t, newConfigClientTLS()) }
func TestV2CurlClientBoth(t *testing.T) { testCurlPutGet(t, newConfigClientBoth()) }
func testCurlPutGet(t *testing.T, cfg *etcdProcessClusterConfig) {
	BeforeTestV2(t)

	// test doesn't use quorum gets, so ensure there are no followers to avoid
	// stale reads that will break the test
	cfg = configStandalone(*cfg)

	cfg.enableV2 = true
	epc, err := newEtcdProcessCluster(t, cfg)
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
	if err := cURLPut(epc, cURLReq{endpoint: "/v2/keys/foo", value: "bar", expected: expectPut}); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}
	if err := cURLGet(epc, cURLReq{endpoint: "/v2/keys/foo", expected: expectGet}); err != nil {
		t.Fatalf("failed get with curl (%v)", err)
	}
	if cfg.clientTLS == clientTLSAndNonTLS {
		if err := cURLGet(epc, cURLReq{endpoint: "/v2/keys/foo", expected: expectGet, isTLS: true}); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	}
}

func TestV2CurlIssue5182(t *testing.T) {
	BeforeTestV2(t)

	copied := newConfigNoTLS()
	copied.enableV2 = true
	epc := setupEtcdctlTest(t, copied, false)
	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", err)
		}
	}()

	expectPut := `{"action":"set","node":{"key":"/foo","value":"bar","`
	if err := cURLPut(epc, cURLReq{endpoint: "/v2/keys/foo", value: "bar", expected: expectPut}); err != nil {
		t.Fatal(err)
	}

	expectUserAdd := `{"user":"foo","roles":null}`
	if err := cURLPut(epc, cURLReq{endpoint: "/v2/auth/users/foo", value: `{"user":"foo", "password":"pass"}`, expected: expectUserAdd}); err != nil {
		t.Fatal(err)
	}
	expectRoleAdd := `{"role":"foo","permissions":{"kv":{"read":["/foo/*"],"write":null}}`
	if err := cURLPut(epc, cURLReq{endpoint: "/v2/auth/roles/foo", value: `{"role":"foo", "permissions": {"kv": {"read": ["/foo/*"]}}}`, expected: expectRoleAdd}); err != nil {
		t.Fatal(err)
	}
	expectUserUpdate := `{"user":"foo","roles":["foo"]}`
	if err := cURLPut(epc, cURLReq{endpoint: "/v2/auth/users/foo", value: `{"user": "foo", "grant": ["foo"]}`, expected: expectUserUpdate}); err != nil {
		t.Fatal(err)
	}

	if err := etcdctlUserAdd(epc, "root", "a"); err != nil {
		t.Fatal(err)
	}
	if err := etcdctlAuthEnable(epc); err != nil {
		t.Fatal(err)
	}

	if err := cURLGet(epc, cURLReq{endpoint: "/v2/keys/foo/", username: "root", password: "a", expected: "bar"}); err != nil {
		t.Fatal(err)
	}
	if err := cURLGet(epc, cURLReq{endpoint: "/v2/keys/foo/", username: "foo", password: "pass", expected: "bar"}); err != nil {
		t.Fatal(err)
	}
	if err := cURLGet(epc, cURLReq{endpoint: "/v2/keys/foo/", username: "foo", password: "", expected: "bar"}); err != nil {
		if !strings.Contains(err.Error(), `The request requires user authentication`) {
			t.Fatalf("expected 'The request requires user authentication' error, got %v", err)
		}
	} else {
		t.Fatalf("expected 'The request requires user authentication' error")
	}
}

type cURLReq struct {
	username string
	password string

	isTLS   bool
	timeout int

	endpoint string

	value    string
	expected string
	header   string

	metricsURLScheme string

	ciphers string
}

// cURLPrefixArgs builds the beginning of a curl command for a given key
// addressed to a random URL in the given cluster.
func cURLPrefixArgs(clus *etcdProcessCluster, method string, req cURLReq) []string {
	var (
		cmdArgs = []string{"curl"}
		acurl   = clus.procs[rand.Intn(clus.cfg.clusterSize)].Config().acurl
	)
	if req.metricsURLScheme != "https" {
		if req.isTLS {
			if clus.cfg.clientTLS != clientTLSAndNonTLS {
				panic("should not use cURLPrefixArgsUseTLS when serving only TLS or non-TLS")
			}
			cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
			acurl = toTLS(clus.procs[rand.Intn(clus.cfg.clusterSize)].Config().acurl)
		} else if clus.cfg.clientTLS == clientTLS {
			if !clus.cfg.noCN {
				cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
			} else {
				cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath3, "--key", privateKeyPath3)
			}
		}
	}
	if req.metricsURLScheme != "" {
		acurl = clus.procs[rand.Intn(clus.cfg.clusterSize)].EndpointsMetrics()[0]
	}
	ep := acurl + req.endpoint

	if req.username != "" || req.password != "" {
		cmdArgs = append(cmdArgs, "-L", "-u", fmt.Sprintf("%s:%s", req.username, req.password), ep)
	} else {
		cmdArgs = append(cmdArgs, "-L", ep)
	}
	if req.timeout != 0 {
		cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%d", req.timeout))
	}

	if req.header != "" {
		cmdArgs = append(cmdArgs, "-H", req.header)
	}

	if req.ciphers != "" {
		cmdArgs = append(cmdArgs, "--ciphers", req.ciphers)
	}

	switch method {
	case "POST", "PUT":
		dt := req.value
		if !strings.HasPrefix(dt, "{") { // for non-JSON value
			dt = "value=" + dt
		}
		cmdArgs = append(cmdArgs, "-X", method, "-d", dt)
	}
	return cmdArgs
}

func cURLPost(clus *etcdProcessCluster, req cURLReq) error {
	return spawnWithExpect(cURLPrefixArgs(clus, "POST", req), req.expected)
}

func cURLPut(clus *etcdProcessCluster, req cURLReq) error {
	return spawnWithExpect(cURLPrefixArgs(clus, "PUT", req), req.expected)
}

func cURLGet(clus *etcdProcessCluster, req cURLReq) error {
	return spawnWithExpect(cURLPrefixArgs(clus, "GET", req), req.expected)
}
