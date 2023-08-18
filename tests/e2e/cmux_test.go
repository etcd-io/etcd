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

// These tests are directly validating etcd connection multiplexing.
//go:build !cluster_proxy

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/etcdserver/api/etcdhttp"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestConnectionMultiplexing(t *testing.T) {
	e2e.BeforeTest(t)
	for _, tc := range []struct {
		name             string
		serverTLS        e2e.ClientConnType
		separateHttpPort bool
	}{
		{
			name:      "ServerTLS",
			serverTLS: e2e.ClientTLS,
		},
		{
			name:      "ServerNonTLS",
			serverTLS: e2e.ClientNonTLS,
		},
		{
			name:      "ServerTLSAndNonTLS",
			serverTLS: e2e.ClientTLSAndNonTLS,
		},
		{
			name:             "SeparateHTTP/ServerTLS",
			serverTLS:        e2e.ClientTLS,
			separateHttpPort: true,
		},
		{
			name:             "SeparateHTTP/ServerNonTLS",
			serverTLS:        e2e.ClientNonTLS,
			separateHttpPort: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := e2e.EtcdProcessClusterConfig{ClusterSize: 1, Client: e2e.ClientConfig{ConnectionType: tc.serverTLS}, ClientHttpSeparate: tc.separateHttpPort}
			clus, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(&cfg))
			require.NoError(t, err)
			defer clus.Close()

			var clientScenarios []e2e.ClientConnType
			switch tc.serverTLS {
			case e2e.ClientTLS:
				clientScenarios = []e2e.ClientConnType{e2e.ClientTLS}
			case e2e.ClientNonTLS:
				clientScenarios = []e2e.ClientConnType{e2e.ClientNonTLS}
			case e2e.ClientTLSAndNonTLS:
				clientScenarios = []e2e.ClientConnType{e2e.ClientTLS, e2e.ClientNonTLS}
			}

			for _, clientTLS := range clientScenarios {
				name := "ClientNonTLS"
				if clientTLS == e2e.ClientTLS {
					name = "ClientTLS"
				}
				t.Run(name, func(t *testing.T) {
					testConnectionMultiplexing(t, ctx, clus.Procs[0], clientTLS)
				})
			}
		})
	}
}

func testConnectionMultiplexing(t *testing.T, ctx context.Context, member e2e.EtcdProcess, connType e2e.ClientConnType) {
	httpEndpoint := member.EndpointsHTTP()[0]
	grpcEndpoint := member.EndpointsGRPC()[0]
	switch connType {
	case e2e.ClientTLS:
		httpEndpoint = e2e.ToTLS(httpEndpoint)
		grpcEndpoint = e2e.ToTLS(grpcEndpoint)
	case e2e.ClientNonTLS:
	default:
		panic(fmt.Sprintf("Unsupported conn type %v", connType))
	}
	t.Run("etcdctl", func(t *testing.T) {
		etcdctl, err := e2e.NewEtcdctl(e2e.ClientConfig{ConnectionType: connType}, []string{grpcEndpoint})
		require.NoError(t, err)
		_, err = etcdctl.Get(ctx, "a", config.GetOptions{})
		assert.NoError(t, err)
	})
	t.Run("clientv3", func(t *testing.T) {
		c := newClient(t, []string{grpcEndpoint}, e2e.ClientConfig{ConnectionType: connType})
		_, err := c.Get(ctx, "a")
		assert.NoError(t, err)
	})
	t.Run("curl", func(t *testing.T) {
		for _, httpVersion := range []string{"2", "1.1", "1.0", ""} {
			tname := "http" + httpVersion
			if httpVersion == "" {
				tname = "default"
			}
			t.Run(tname, func(t *testing.T) {
				assert.NoError(t, fetchGrpcGateway(httpEndpoint, httpVersion, connType))
				assert.NoError(t, fetchMetrics(t, httpEndpoint, httpVersion, connType))
				assert.NoError(t, fetchVersion(httpEndpoint, httpVersion, connType))
				assert.NoError(t, fetchHealth(httpEndpoint, httpVersion, connType))
				assert.NoError(t, fetchDebugVars(httpEndpoint, httpVersion, connType))
			})
		}
	})
}

func fetchGrpcGateway(endpoint string, httpVersion string, connType e2e.ClientConnType) error {
	rangeData, err := json.Marshal(&pb.RangeRequest{
		Key: []byte("a"),
	})
	if err != nil {
		return err
	}
	req := e2e.CURLReq{Endpoint: "/v3/kv/range", Value: string(rangeData), Timeout: 5, HttpVersion: httpVersion}
	respData, err := curl(endpoint, "POST", req, connType)
	if err != nil {
		return err
	}
	return validateGrpcgatewayRangeReponse([]byte(respData))
}

func validateGrpcgatewayRangeReponse(respData []byte) error {
	// Modified json annotation so ResponseHeader fields are stored in string.
	type responseHeader struct {
		ClusterId uint64 `json:"cluster_id,string,omitempty"`
		MemberId  uint64 `json:"member_id,string,omitempty"`
		Revision  int64  `json:"revision,string,omitempty"`
		RaftTerm  uint64 `json:"raft_term,string,omitempty"`
	}
	type rangeResponse struct {
		Header *responseHeader    `json:"header,omitempty"`
		Kvs    []*mvccpb.KeyValue `json:"kvs,omitempty"`
		More   bool               `json:"more,omitempty"`
		Count  int64              `json:"count,omitempty"`
	}
	var resp rangeResponse
	return json.Unmarshal(respData, &resp)
}

func fetchMetrics(t *testing.T, endpoint string, httpVersion string, connType e2e.ClientConnType) error {
	tmpDir := t.TempDir()
	metricFile := filepath.Join(tmpDir, "metrics")

	req := e2e.CURLReq{Endpoint: "/metrics", Timeout: 5, HttpVersion: httpVersion, OutputFile: metricFile}
	if _, err := curl(endpoint, "GET", req, connType); err != nil {
		return err
	}

	rawData, err := os.ReadFile(metricFile)
	if err != nil {
		return fmt.Errorf("failed to read the metric: %w", err)
	}
	respData := string(rawData)

	var parser expfmt.TextParser
	_, err = parser.TextToMetricFamilies(strings.NewReader(strings.ReplaceAll(respData, "\r\n", "\n")))
	return err
}

func fetchVersion(endpoint string, httpVersion string, connType e2e.ClientConnType) error {
	req := e2e.CURLReq{Endpoint: "/version", Timeout: 5, HttpVersion: httpVersion}
	respData, err := curl(endpoint, "GET", req, connType)
	if err != nil {
		return err
	}
	var resp version.Versions
	return json.Unmarshal([]byte(respData), &resp)
}

func fetchHealth(endpoint string, httpVersion string, connType e2e.ClientConnType) error {
	req := e2e.CURLReq{Endpoint: "/health", Timeout: 5, HttpVersion: httpVersion}
	respData, err := curl(endpoint, "GET", req, connType)
	if err != nil {
		return err
	}
	var resp etcdhttp.Health
	return json.Unmarshal([]byte(respData), &resp)
}

func fetchDebugVars(endpoint string, httpVersion string, connType e2e.ClientConnType) error {
	req := e2e.CURLReq{Endpoint: "/debug/vars", Timeout: 5, HttpVersion: httpVersion}
	respData, err := curl(endpoint, "GET", req, connType)
	if err != nil {
		return err
	}
	var resp map[string]interface{}
	return json.Unmarshal([]byte(respData), &resp)
}
