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
// +build !cluster_proxy

// TODO: fix race conditions with setupLogging

package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/embed"
	gofail "go.etcd.io/gofail/runtime"
)

func TestEmbedEtcd(t *testing.T) {
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
		tests[i].cfg.Debug = false
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

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("embed-etcd"))
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

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
			t.Errorf("#%d: unexpected error on close (%v)", i, err)
		default:
		}
	}
}

func TestEmbedEtcdGracefulStopSecure(t *testing.T)   { testEmbedEtcdGracefulStop(t, true) }
func TestEmbedEtcdGracefulStopInsecure(t *testing.T) { testEmbedEtcdGracefulStop(t, false) }

// testEmbedEtcdGracefulStop ensures embedded server stops
// cutting existing transports.
func testEmbedEtcdGracefulStop(t *testing.T, secure bool) {
	cfg := embed.NewConfig()
	if secure {
		cfg.ClientTLSInfo = testTLSInfo
		cfg.PeerTLSInfo = testTLSInfo
	}

	urls := newEmbedURLs(secure, 2)
	setupEmbedCfg(cfg, []url.URL{urls[0]}, []url.URL{urls[1]})

	cfg.Dir = filepath.Join(os.TempDir(), fmt.Sprintf("embed-etcd"))
	os.RemoveAll(cfg.Dir)
	defer os.RemoveAll(cfg.Dir)

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
	cli, err := clientv3.New(clientCfg)
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
	case err := <-e.Err():
		t.Fatal(err)
	case <-donec:
	case <-time.After(2*time.Second + e.Server.Cfg.ReqTimeout()):
		t.Fatalf("took too long to close server")
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
	cfg.Debug = false

	cfg.ClusterState = "new"
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = curls, curls
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = purls, purls
	cfg.InitialCluster = ""
	for i := range purls {
		cfg.InitialCluster += ",default=" + purls[i].String()
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
}

func TestEmbedEtcdStopDuringBootstrapping(t *testing.T) {
	if len(gofail.List()) == 0 {
		t.Skip("please run 'make gofail-enable' before running the test")
	}

	fpName := "beforePublishing"
	require.NoError(t, gofail.Enable(fpName, `sleep("2s")`))
	t.Cleanup(func() {
		terr := gofail.Disable(fpName)
		if terr != nil && terr != gofail.ErrDisabled {
			t.Fatalf("failed to disable %s: %v", fpName, terr)
		}
	})

	done := make(chan struct{})
	go func() {
		defer close(done)

		cfg := embed.NewConfig()
		urls := newEmbedURLs(false, 2)
		setupEmbedCfg(cfg, []url.URL{urls[0]}, []url.URL{urls[1]})
		cfg.Dir = filepath.Join(t.TempDir(), "embed-etcd")

		e, err := embed.StartEtcd(cfg)
		if err != nil {
			t.Errorf("Failed to start etcd, got error %v", err)
		}
		defer e.Close()

		go func() {
			time.Sleep(time.Second)
			e.Server.Stop()
			t.Log("Stopped server during bootstrapping")
		}()

		select {
		case <-e.Server.ReadyNotify():
			t.Log("Server is ready!")
		case <-e.Server.StopNotify():
			t.Log("Server is stopped")
		case <-time.After(20 * time.Second):
			e.Server.Stop() // trigger a shutdown
			t.Error("Server took too long to start!")
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Error("timeout in bootstrapping etcd")
	}
}
