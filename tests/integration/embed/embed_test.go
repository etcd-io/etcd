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

//go:build !cluster_proxy

// Keep the test in a separate package from other tests such that
// .setupLogging method does not race with other (previously running) servers (grpclog is global).

package embed_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

var (
	testTLSInfo = transport.TLSInfo{
		KeyFile:        testutils.MustAbsPath("../../fixtures/server.key.insecure"),
		CertFile:       testutils.MustAbsPath("../../fixtures/server.crt"),
		TrustedCAFile:  testutils.MustAbsPath("../../fixtures/ca.crt"),
		ClientCertAuth: true,
	}
)

func TestEmbedEtcd(t *testing.T) {
	testutil.SkipTestIfShortMode(t, "Cannot start embedded cluster in --short tests")

	tests := []struct {
		cfg embed.Config

		werr     string
		wpeers   int
		wclients int
	}{
		{werr: "multiple discovery"},
		{werr: "advertise-client-urls is required"},
		{werr: "should be at least"},
		{werr: "is too long"},
		{wpeers: 1, wclients: 1},
		{wpeers: 2, wclients: 1},
		{wpeers: 1, wclients: 2},
		{werr: "expected IP"},
		{werr: "expected IP"},
	}

	urls := newEmbedURLs(false, 10)

	// setup defaults
	for i := range tests {
		tests[i].cfg = *embed.NewConfig()
		tests[i].cfg.Logger = "zap"
		tests[i].cfg.LogOutputs = []string{"/dev/null"}
	}

	tests[0].cfg.Durl = "abc"
	setupEmbedCfg(&tests[1].cfg, []url.URL{urls[0]}, []url.URL{urls[1]})
	tests[1].cfg.AdvertiseClientUrls = nil
	tests[2].cfg.TickMs = tests[2].cfg.ElectionMs - 1
	tests[3].cfg.ElectionMs = 999999
	setupEmbedCfg(&tests[4].cfg, []url.URL{urls[2]}, []url.URL{urls[3]})
	setupEmbedCfg(&tests[5].cfg, []url.URL{urls[4]}, []url.URL{urls[5], urls[6]})
	setupEmbedCfg(&tests[6].cfg, []url.URL{urls[7], urls[8]}, []url.URL{urls[9]})

	dnsURL, _ := url.Parse("http://whatever.test:12345")
	tests[7].cfg.ListenClientUrls = []url.URL{*dnsURL}
	tests[8].cfg.ListenPeerUrls = []url.URL{*dnsURL}

	dir := filepath.Join(t.TempDir(), "embed-etcd")

	for i, tt := range tests {
		tests[i].cfg.Dir = dir
		e, err := embed.StartEtcd(&tests[i].cfg)
		if e != nil {
			<-e.Server.ReadyNotify() // wait for e.Server to join the cluster
		}
		if tt.werr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.werr) {
				t.Errorf("%d: expected error with %q, got %v", i, tt.werr, err)
			}
			if e != nil {
				e.Close()
			}
			continue
		}
		if err != nil {
			t.Errorf("%d: expected success, got error %v", i, err)
			continue
		}
		if len(e.Peers) != tt.wpeers {
			t.Errorf("%d: expected %d peers, got %d", i, tt.wpeers, len(e.Peers))
		}
		if len(e.Clients) != tt.wclients {
			t.Errorf("%d: expected %d clients, got %d", i, tt.wclients, len(e.Clients))
		}
		e.Close()
		select {
		case err := <-e.Err():
			if err != nil {
				t.Errorf("#%d: unexpected error on close (%v)", i, err)
			}
		}
	}
}

func TestEmbedEtcdGracefulStopSecure(t *testing.T)   { testEmbedEtcdGracefulStop(t, true) }
func TestEmbedEtcdGracefulStopInsecure(t *testing.T) { testEmbedEtcdGracefulStop(t, false) }

// testEmbedEtcdGracefulStop ensures embedded server stops
// cutting existing transports.
func testEmbedEtcdGracefulStop(t *testing.T, secure bool) {
	testutil.SkipTestIfShortMode(t, "Cannot start embedded cluster in --short tests")

	cfg := embed.NewConfig()
	if secure {
		cfg.ClientTLSInfo = testTLSInfo
		cfg.PeerTLSInfo = testTLSInfo
	}

	urls := newEmbedURLs(secure, 2)
	setupEmbedCfg(cfg, []url.URL{urls[0]}, []url.URL{urls[1]})

	cfg.Dir = filepath.Join(t.TempDir(), "embed-etcd")

	e, err := embed.StartEtcd(cfg)
	if err != nil {
		t.Fatal(err)
	}
	<-e.Server.ReadyNotify() // wait for e.Server to join the cluster

	clientCfg := clientv3.Config{
		Endpoints: []string{urls[0].String()},
	}
	if secure {
		clientCfg.TLS, err = testTLSInfo.ClientConfig()
		if err != nil {
			t.Fatal(err)
		}
	}
	cli, err := integration2.NewClient(t, clientCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// open watch connection
	cli.Watch(context.Background(), "foo")

	donec := make(chan struct{})
	go func() {
		e.Close()
		close(donec)
	}()
	select {
	case <-donec:
	case <-time.After(2*time.Second + e.Server.Cfg.ReqTimeout()):
		t.Fatalf("took too long to close server")
	}
	err = <-e.Err()
	if err != nil {
		t.Fatal(err)
	}
}

//func TestEmbedEtcdClusterGracefulStopSecure(t *testing.T) {
//	testEmbedEtcdClusterGracefulStop(t, true)
//}

func TestEmbedEtcdClusterGracefulStopInsecure(t *testing.T) {
	testEmbedEtcdClusterGracefulStop(t, false)
}

func testEmbedEtcdClusterGracefulStop(t *testing.T, secure bool) {
	testutil.SkipTestIfShortMode(t, "Cannot start embedded cluster in --short tests")

	nodes := 3 // need to be an odd number larger than 1

	urls := newEmbedURLs(secure, nodes*2)

	// start a cluster
	var err error
	cfg := make([]*embed.Config, nodes)
	e := make([]*embed.Etcd, nodes)
	for i := 0; i < nodes; i++ {
		cfg[i] = setupEmbedClusterCfg(t, secure, urls[0:nodes], urls[nodes:], i)
		e[i], err = embed.StartEtcd(cfg[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	// wait for all nodes to be ready
	for i := 0; i < nodes; i++ {
		select {
		case <-e[i].Server.ReadyNotify(): // wait for e.Server to join the cluster
		case <-time.After(10 * time.Second):
			t.Fatalf("node %d did not join the cluster in time", i)
		}
	}

	// make sure all nodes joined the cluster and are not learners
	requireHealthClusterEventually(t, e, 50*time.Millisecond, time.Second, secure)

	// stop one node
	e[nodes-1].Close()

	// remove the stopped node from the cluster
	removeMember(t, e[0], e[nodes-1].Server.MemberId(), secure)

	// start the node again -> fails as it was removed from the cluster
	cfg[nodes-1] = setupEmbedClusterCfg(t, secure, urls[0:nodes], urls[nodes:], nodes-1)
	e[nodes-1], err = embed.StartEtcd(cfg[nodes-1])
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-e[nodes-1].Server.ReadyNotify():
		t.Fatal("the start should have failed")
	case <-e[nodes-1].Server.StopNotify():
	}

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		for i := 0; i < nodes; i++ {
			e[i].Close()
		}
	}()
	select {
	case <-donec:
	case <-time.After(2*time.Second + e[0].Server.Cfg.ReqTimeout()):
		t.Fatalf("took too long to close servers")
	}
}

func TestEmbedEtcdClusterStartPartialInsecure(t *testing.T) {
	testEmbedEtcdClusterStartPartial(t, false)
}

func testEmbedEtcdClusterStartPartial(t *testing.T, secure bool) {
	testutil.SkipTestIfShortMode(t, "Cannot start embedded cluster in --short tests")

	nodes := 3 // need to be an odd number larger than 1

	urls := newEmbedURLs(secure, nodes*2)

	// set up a cluster but only start the first node
	cfg := make([]*embed.Config, nodes)
	for i := 0; i < nodes; i++ {
		cfg[i] = setupEmbedClusterCfg(t, secure, urls[0:nodes], urls[nodes:], i)
	}
	e, err := embed.StartEtcd(cfg[0])
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-e.Server.ReadyNotify():
		t.Fatal("server should not have been ready")
	case <-e.Server.StopNotify():
		t.Fatal("server should not have been stopped")
	case <-time.After(time.Second):
	}

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		e.Close()
	}()
	select {
	case <-donec:
	case <-time.After(2*time.Second + e.Server.Cfg.ReqTimeout()):
		printCallstacks()
		t.Fatalf("took too long to close servers")
	}
}

func newEmbedURLs(secure bool, n int) (urls []url.URL) {
	scheme := "unix"
	if secure {
		scheme = "unixs"
	}
	for i := 0; i < n; i++ {
		u, _ := url.Parse(fmt.Sprintf("%s://localhost:%d%06d", scheme, os.Getpid(), i))
		urls = append(urls, *u)
	}
	return urls
}

func setupEmbedCfg(cfg *embed.Config, curls []url.URL, purls []url.URL) {
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}

	cfg.ClusterState = "new"
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = curls, curls
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = purls, purls
	cfg.InitialCluster = ""
	for i := range purls {
		cfg.InitialCluster += ",default=" + purls[i].String()
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
}

func setupEmbedClusterCfg(t *testing.T, secure bool, curls []url.URL, purls []url.URL, n int) *embed.Config {
	cfg := embed.NewConfig()
	if secure {
		cfg.ClientTLSInfo = testTLSInfo
		cfg.PeerTLSInfo = testTLSInfo
	}

	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}

	cfg.Name = fmt.Sprintf("node-%d", n)
	cfg.ClusterState = "new"
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = []url.URL{curls[n]}, []url.URL{curls[n]}
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = []url.URL{purls[n]}, []url.URL{purls[n]}
	cfg.InitialCluster = ""
	for i := range purls {
		cfg.InitialCluster += fmt.Sprintf(",node-%d=%s", i, purls[i].String())
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
	cfg.Dir = filepath.Join(t.TempDir(), fmt.Sprintf("embed-etcd-%d", n))

	return cfg
}

func requireHealthClusterEventually(t *testing.T, instances []*embed.Etcd, checkPeriod time.Duration, timeout time.Duration, secure bool) {
	var clients []*clientv3.Client
	defer func() {
		for _, client := range clients {
			assert.NoError(t, client.Close())
		}
	}()

	for _, instance := range instances {
		clientCfg := clientv3.Config{
			Endpoints: []string{instance.Config().AdvertiseClientUrls[0].String()},
		}
		if secure {
			var err error
			clientCfg.TLS, err = testTLSInfo.ClientConfig()
			require.NoError(t, err)
		}
		cli, err := integration2.NewClient(t, clientCfg)
		require.NoError(t, err)
		clients = append(clients, cli)
	}

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(context.Background(), timeout)
	defer timeoutCtxCancel()

	for {
		select {
		case <-time.After(checkPeriod):
			allPassed := true
			for _, client := range clients {
				if !isHealthy(client) {
					allPassed = false
					break
				}
			}
			if allPassed {
				return
			}
		case <-timeoutCtx.Done():
			t.Fatal("nodes did not become members in time")
		}
	}
}

// perform a similar test as what "etcdctl endpoint health" does
func isHealthy(cli *clientv3.Client) bool {
	ctx, ctxCanel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer ctxCanel()

	// get a random key. As long as we can get the response without an error, the endpoint is health.
	_, err := cli.Get(ctx, "health")
	// permission denied is OK since proposal goes through consensus to get it
	return err == nil || err == rpctypes.ErrPermissionDenied
}

func removeMember(t *testing.T, instance *embed.Etcd, memberID types.ID, secure bool) {
	ctx, ctxCanel := context.WithTimeout(context.Background(), time.Second)
	defer ctxCanel()

	clientCfg := clientv3.Config{
		Endpoints: []string{instance.Config().AdvertiseClientUrls[0].String()},
	}
	if secure {
		var err error
		clientCfg.TLS, err = testTLSInfo.ClientConfig()
		require.NoError(t, err)
	}
	cli, err := integration2.NewClient(t, clientCfg)
	require.NoError(t, err)
	defer cli.Close()

	_, err = cli.MemberRemove(ctx, uint64(memberID))
	require.NoError(t, err)
}

func TestEmbedEtcdAutoCompactionRetentionRetained(t *testing.T) {
	cfg := embed.NewConfig()
	urls := newEmbedURLs(false, 2)
	setupEmbedCfg(cfg, []url.URL{urls[0]}, []url.URL{urls[1]})
	cfg.Dir = filepath.Join(t.TempDir(), "embed-etcd")

	cfg.AutoCompactionRetention = "2"

	e, err := embed.StartEtcd(cfg)
	assert.NoError(t, err)
	autoCompactionRetention := e.Server.Cfg.AutoCompactionRetention
	duration_to_compare, _ := time.ParseDuration("2h0m0s")
	assert.Equal(t, duration_to_compare, autoCompactionRetention)
	e.Close()
}

func printCallstacks() {
	buf := make([]byte, 64*1024)
	bytes := runtime.Stack(buf, true)

	println(string(buf[0:bytes]))
}
