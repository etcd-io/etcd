// Copyright 2019 The etcd Authors
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

package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"go.etcd.io/etcd/discoveryserver/handlers"
	discoveryhttp "go.etcd.io/etcd/discoveryserver/http"

	"go.etcd.io/etcd/embed"
	"go.etcd.io/etcd/etcdserver/api/v3client"
)

// Service contains test discovery server components.
type Service struct {
	rootCtx    context.Context
	rootCancel func()

	cfg      *embed.Config
	dataDir  string
	etcdCURL url.URL
	etcd     *embed.Etcd

	httpEp     string
	httpServer *http.Server
	httpErrc   chan error
	state      *handlers.State
}

const testDiscoveryHost = "handler-test"

// NewService creates a new service.
func NewService(t *testing.T, etcdClientPort, etcdPeerPort, httpPort int) *Service {
	dataDir, err := ioutil.TempDir(os.TempDir(), "test-data")
	if err != nil {
		t.Fatal(err)
	}

	cfg := embed.NewConfig()
	cfg.ClusterState = embed.ClusterStateFlagNew
	cfg.Name = "test-etcd"
	cfg.Dir = dataDir
	curl := url.URL{Scheme: "http", Host: fmt.Sprintf("localhost:%d", etcdClientPort)}
	cfg.ACUrls, cfg.LCUrls = []url.URL{curl}, []url.URL{curl}
	purl := url.URL{Scheme: "http", Host: fmt.Sprintf("localhost:%d", etcdPeerPort)}
	cfg.APUrls, cfg.LPUrls = []url.URL{purl}, []url.URL{purl}
	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, cfg.APUrls[0].String())

	// TODO: enable this with etcd v3.3+
	// cfg.AutoCompactionMode = compactor.ModePeriodic
	// cfg.AutoCompactionRetention = 1

	ctx, cancel := context.WithCancel(context.Background())
	h, state := discoveryhttp.RegisterHandlers(ctx, cfg.LCUrls[0].String(), testDiscoveryHost)
	return &Service{
		rootCtx:    ctx,
		rootCancel: cancel,

		cfg:      cfg,
		dataDir:  dataDir,
		etcdCURL: curl,

		httpEp: fmt.Sprintf("http://localhost:%d", httpPort),
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("localhost:%d", httpPort),
			Handler: h,
		},
		httpErrc: make(chan error),
		state:    state,
	}
}

// Start starts etcd server and http listener.
func (sv *Service) Start(t *testing.T) <-chan error {
	srv, err := embed.StartEtcd(sv.cfg)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-srv.Server.ReadyNotify():
		err = nil
	case err = <-srv.Err():
	case <-srv.Server.StopNotify():
		err = fmt.Errorf("received from etcdserver.Server.StopNotify")
	}
	if err != nil {
		t.Fatal(err)
	}
	sv.etcd = srv

	// issue linearized read to ensure leader election
	cli := v3client.New(srv.Server)
	_, _ = cli.Get(context.Background(), "foo")

	go func() {
		defer close(sv.httpErrc)
		if err := sv.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sv.httpErrc <- err
			return
		}
		sv.httpErrc <- nil
	}()
	return sv.httpErrc
}

// Stop stops etcd server, removing the data directory, and http server.
func (sv *Service) Stop(t *testing.T) {
	defer os.RemoveAll(sv.dataDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := sv.httpServer.Shutdown(ctx)
	cancel()
	if err != nil && err != context.DeadlineExceeded {
		t.Fatal(err)
	}
	err = <-sv.httpErrc
	if err != nil {
		t.Fatal(err)
	}

	sv.rootCancel()
	sv.etcd.Close()
}
