// Copyright 2022 The etcd Authors
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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/v2"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestClusterOf1UsingDiscovery(t *testing.T)    { testClusterUsingDiscovery(t, 1, false) }
func TestClusterOf3UsingDiscovery(t *testing.T)    { testClusterUsingDiscovery(t, 3, false) }
func TestTLSClusterOf3UsingDiscovery(t *testing.T) { testClusterUsingDiscovery(t, 3, true) }

func testClusterUsingDiscovery(t *testing.T, size int, peerTLS bool) {
	e2e.BeforeTest(t)

	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	dc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		BasePort:    2000,
		ExecPath:    lastReleaseBinary,
		ClusterSize: 1,
		EnableV2:    true,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer dc.Close()

	dcc := MustNewHTTPClient(t, dc.EndpointsV2(), nil)
	dkapi := client.NewKeysAPI(dcc)
	ctx, cancel := context.WithTimeout(context.Background(), integration.RequestTimeout)
	if _, err := dkapi.Create(ctx, "/_config/size", fmt.Sprintf("%d", size)); err != nil {
		t.Fatal(err)
	}
	cancel()

	c, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		BasePort:    3000,
		ClusterSize: size,
		IsPeerTLS:   peerTLS,
		Discovery:   dc.EndpointsV2()[0] + "/v2/keys",
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer c.Close()

	kubectl := []string{e2e.CtlBinPath, "--endpoints", strings.Join(c.EndpointsV3(), ",")}
	if err := e2e.SpawnWithExpect(append(kubectl, "put", "key", "value"), "OK"); err != nil {
		t.Fatal(err)
	}
	if err := e2e.SpawnWithExpect(append(kubectl, "get", "key"), "value"); err != nil {
		t.Fatal(err)
	}
}

func MustNewHTTPClient(t testutil.TB, eps []string, tls *transport.TLSInfo) client.Client {
	cfgtls := transport.TLSInfo{}
	if tls != nil {
		cfgtls = *tls
	}
	cfg := client.Config{Transport: mustNewTransport(t, cfgtls), Endpoints: eps}
	c, err := client.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustNewTransport(t testutil.TB, tlsInfo transport.TLSInfo) *http.Transport {
	// tick in integration test is short, so 1s dial timeout could play well.
	tr, err := transport.NewTimeoutTransport(tlsInfo, time.Second, rafthttp.ConnReadTimeout, rafthttp.ConnWriteTimeout)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}
