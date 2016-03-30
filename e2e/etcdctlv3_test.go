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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/testutil"
)

func TestCtlV3Set(t *testing.T) {
	testCtlV3Set(t, &configNoTLS, 3*time.Second, false)
}

func TestCtlV3SetZeroTimeout(t *testing.T) {
	testCtlV3Set(t, &configNoTLS, 0, false)
}

func TestCtlV3SetTimeout(t *testing.T) {
	testCtlV3Set(t, &configNoTLS, time.Nanosecond, false)
}

func TestCtlV3SetPeerTLS(t *testing.T) {
	testCtlV3Set(t, &configPeerTLS, 3*time.Second, false)
}

func TestCtlV3SetQuorum(t *testing.T) {
	testCtlV3Set(t, &configNoTLS, 3*time.Second, true)
}

func TestCtlV3SetQuorumZeroTimeout(t *testing.T) {
	testCtlV3Set(t, &configNoTLS, 0, true)
}

func TestCtlV3SetQuorumTimeout(t *testing.T) {
	testCtlV3Set(t, &configNoTLS, time.Nanosecond, true)
}

func TestCtlV3SetPeerTLSQuorum(t *testing.T) {
	testCtlV3Set(t, &configPeerTLS, 3*time.Second, true)
}

func testCtlV3Set(t *testing.T, cfg *etcdProcessClusterConfig, dialTimeout time.Duration, quorum bool) {
	defer testutil.AfterTest(t)

	os.Setenv("ETCDCTL_API", "3")
	epc := setupCtlV3Test(t, cfg, quorum)
	defer func() {
		os.Unsetenv("ETCDCTL_API")
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	key, value := "foo", "bar"

	errc := make(chan error, 1)
	expectTimeout := dialTimeout > 0 && dialTimeout <= time.Nanosecond
	go func() {
		defer close(errc)
		if err := ctlV3Put(epc, key, value, dialTimeout); err != nil {
			if expectTimeout && isGRPCTimedout(err) {
				errc <- fmt.Errorf("put error (%v)", err)
				return
			}
		}
		if err := ctlV3Get(epc, key, value, dialTimeout, quorum); err != nil {
			if expectTimeout && isGRPCTimedout(err) {
				errc <- fmt.Errorf("get error (%v)", err)
				return
			}
		}
	}()

	select {
	case <-time.After(2*dialTimeout + time.Second):
		if dialTimeout > 0 {
			t.Fatalf("test timed out for %v", dialTimeout)
		}
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ctlV3PrefixArgs(clus *etcdProcessCluster, dialTimeout time.Duration) []string {
	if len(clus.proxies()) > 0 { // TODO: add proxy check as in v2
		panic("v3 proxy not implemented")
	}

	endpoints := ""
	if backends := clus.backends(); len(backends) != 0 {
		es := []string{}
		for _, b := range backends {
			es = append(es, stripSchema(b.cfg.acurl))
		}
		endpoints = strings.Join(es, ",")
	}
	cmdArgs := []string{"../bin/etcdctl", "--endpoints", endpoints, "--dial-timeout", dialTimeout.String()}
	if clus.cfg.clientTLS == clientTLS {
		cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
	}
	return cmdArgs
}

func ctlV3Put(clus *etcdProcessCluster, key, value string, dialTimeout time.Duration) error {
	cmdArgs := append(ctlV3PrefixArgs(clus, dialTimeout), "put", key, value)
	return spawnWithExpect(cmdArgs, "OK")
}

func ctlV3Get(clus *etcdProcessCluster, key, value string, dialTimeout time.Duration, quorum bool) error {
	cmdArgs := append(ctlV3PrefixArgs(clus, dialTimeout), "get", key)
	if !quorum {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}
	// TODO: match by value. Currently it prints out both key and value in multi-lines.
	return spawnWithExpect(cmdArgs, key)
}

func setupCtlV3Test(t *testing.T, cfg *etcdProcessClusterConfig, quorum bool) *etcdProcessCluster {
	mustEtcdctl(t)
	if !quorum {
		cfg = configStandalone(*cfg)
	}
	copied := *cfg
	epc, err := newEtcdProcessCluster(&copied)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}

func isGRPCTimedout(err error) bool {
	return strings.Contains(err.Error(), "grpc: timed out trying to connect")
}

func stripSchema(s string) string {
	if strings.HasPrefix(s, "http://") {
		s = strings.Replace(s, "http://", "", -1)
	}
	if strings.HasPrefix(s, "https://") {
		s = strings.Replace(s, "https://", "", -1)
	}
	return s
}
