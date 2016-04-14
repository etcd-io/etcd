// Copyright 2016 CoreOS, Inc.
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
	"math/rand"
	"testing"

	"github.com/coreos/etcd/pkg/testutil"
)

func TestV2CurlNoTLS(t *testing.T)        { testCurlPutGet(t, &configNoTLS) }
func TestV2CurlAutoTLS(t *testing.T)      { testCurlPutGet(t, &configAutoTLS) }
func TestV2CurlAllTLS(t *testing.T)       { testCurlPutGet(t, &configTLS) }
func TestV2CurlPeerTLS(t *testing.T)      { testCurlPutGet(t, &configPeerTLS) }
func TestV2CurlClientTLS(t *testing.T)    { testCurlPutGet(t, &configClientTLS) }
func TestV2CurlProxyNoTLS(t *testing.T)   { testCurlPutGet(t, &configWithProxy) }
func TestV2CurlProxyTLS(t *testing.T)     { testCurlPutGet(t, &configWithProxyTLS) }
func TestV2CurlProxyPeerTLS(t *testing.T) { testCurlPutGet(t, &configWithProxyPeerTLS) }
func TestV2CurlClientBoth(t *testing.T)   { testCurlPutGet(t, &configClientBoth) }

func testCurlPutGet(t *testing.T, cfg *etcdProcessClusterConfig) {
	defer testutil.AfterTest(t)

	// test doesn't use quorum gets, so ensure there are no followers to avoid
	// stale reads that will break the test
	cfg = configStandalone(*cfg)

	epc, err := newEtcdProcessCluster(cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", err)
		}
	}()

	expectPut := `{"action":"set","node":{"key":"/testKey","value":"foo","`
	expectGet := `{"action":"get","node":{"key":"/testKey","value":"foo","`

	if cfg.clientTLS == clientTLSAndNonTLS {
		if err := cURLPut(epc, "testKey", "foo", expectPut); err != nil {
			t.Fatalf("failed put with curl (%v)", err)
		}

		if err := cURLGet(epc, "testKey", expectGet); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
		if err := cURLGetUseTLS(epc, "testKey", expectGet); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	} else {
		if err := cURLPut(epc, "testKey", "foo", expectPut); err != nil {
			t.Fatalf("failed put with curl (%v)", err)
		}

		if err := cURLGet(epc, "testKey", expectGet); err != nil {
			t.Fatalf("failed get with curl (%v)", err)
		}
	}
}

// cURLPrefixArgs builds the beginning of a curl command for a given key
// addressed to a random URL in the given cluster.
func cURLPrefixArgs(clus *etcdProcessCluster, key string) []string {
	cmdArgs := []string{"curl"}
	acurl := clus.procs[rand.Intn(clus.cfg.clusterSize)].cfg.acurl

	if clus.cfg.clientTLS == clientTLS {
		cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
	}
	keyURL := acurl + "/v2/keys/testKey"
	cmdArgs = append(cmdArgs, "-L", keyURL)
	return cmdArgs
}

func cURLPrefixArgsUseTLS(clus *etcdProcessCluster, key string) []string {
	cmdArgs := []string{"curl"}
	if clus.cfg.clientTLS != clientTLSAndNonTLS {
		panic("should not use cURLPrefixArgsUseTLS when serving only TLS or non-TLS")
	}
	cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
	acurl := clus.procs[rand.Intn(clus.cfg.clusterSize)].cfg.acurltls
	keyURL := acurl + "/v2/keys/testKey"
	cmdArgs = append(cmdArgs, "-L", keyURL)
	return cmdArgs
}

func cURLPut(clus *etcdProcessCluster, key, val, expected string) error {
	args := append(cURLPrefixArgs(clus, key), "-XPUT", "-d", "value="+val)
	return spawnWithExpect(args, expected)
}

func cURLGet(clus *etcdProcessCluster, key, expected string) error {
	return spawnWithExpect(cURLPrefixArgs(clus, key), expected)
}

func cURLGetUseTLS(clus *etcdProcessCluster, key, expected string) error {
	return spawnWithExpect(cURLPrefixArgsUseTLS(clus, key), expected)
}
