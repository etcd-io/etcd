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
	"strings"
	"testing"

	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/version"
	clientv2 "go.etcd.io/etcd/client/v2"
	"go.etcd.io/etcd/server/v3/etcdserver/api/etcdhttp"
)

func TestConnectionMultiplexing(t *testing.T) {
	BeforeTest(t)
	for _, tc := range []struct {
		name             string
		serverTLS        clientConnType
		separateHttpPort bool
	}{
		{
			name:      "ServerTLS",
			serverTLS: clientTLS,
		},
		{
			name:      "ServerNonTLS",
			serverTLS: clientNonTLS,
		},
		{
			name:      "ServerTLSAndNonTLS",
			serverTLS: clientTLSAndNonTLS,
		},
		{
			name:             "SeparateHTTP/ServerTLS",
			serverTLS:        clientTLS,
			separateHttpPort: true,
		},
		{
			name:             "SeparateHTTP/ServerNonTLS",
			serverTLS:        clientNonTLS,
			separateHttpPort: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := etcdProcessClusterConfig{clusterSize: 1, clientTLS: tc.serverTLS, enableV2: true, clientHttpSeparate: tc.separateHttpPort}
			clus, err := newEtcdProcessCluster(t, &cfg)
			require.NoError(t, err)
			defer clus.Close()

			var clientScenarios []clientConnType
			switch tc.serverTLS {
			case clientTLS:
				clientScenarios = []clientConnType{clientTLS}
			case clientNonTLS:
				clientScenarios = []clientConnType{clientNonTLS}
			case clientTLSAndNonTLS:
				clientScenarios = []clientConnType{clientTLS, clientNonTLS}
			}

			for _, connType := range clientScenarios {
				name := "ClientNonTLS"
				if connType == clientTLS {
					name = "ClientTLS"
				}
				t.Run(name, func(t *testing.T) {
					testConnectionMultiplexing(ctx, t, clus.procs[0], connType)
				})
			}
		})
	}
}

func testConnectionMultiplexing(ctx context.Context, t *testing.T, member etcdProcess, connType clientConnType) {
	httpEndpoint := member.EndpointsHTTP()[0]
	grpcEndpoint := member.EndpointsGRPC()[0]
	switch connType {
	case clientTLS:
		httpEndpoint = toTLS(httpEndpoint)
		grpcEndpoint = toTLS(grpcEndpoint)
	case clientNonTLS:
	default:
		panic(fmt.Sprintf("Unsupported conn type %v", connType))
	}
	t.Run("etcdctl", func(t *testing.T) {
		t.Run("v2", func(t *testing.T) {
			etcdctl := NewEtcdctl([]string{httpEndpoint}, connType, false, true)
			err := etcdctl.Set("a", "1")
			assert.NoError(t, err)
		})
		t.Run("v3", func(t *testing.T) {
			etcdctl := NewEtcdctl([]string{grpcEndpoint}, connType, false, false)
			err := etcdctl.Put("a", "1")
			assert.NoError(t, err)
		})
	})
	t.Run("clientv2", func(t *testing.T) {
		c, err := newClientV2(t, []string{httpEndpoint}, connType, false)
		require.NoError(t, err)
		kv := clientv2.NewKeysAPI(c)
		_, err = kv.Set(ctx, "a", "1", nil)
		assert.NoError(t, err)
	})
	t.Run("clientv3", func(t *testing.T) {
		c := newClient(t, []string{grpcEndpoint}, connType, false)
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
				assert.NoError(t, fetchMetrics(httpEndpoint, httpVersion, connType))
				assert.NoError(t, fetchVersion(httpEndpoint, httpVersion, connType))
				assert.NoError(t, fetchHealth(httpEndpoint, httpVersion, connType))
				assert.NoError(t, fetchDebugVars(httpEndpoint, httpVersion, connType))
			})
		}
	})
}

func fetchGrpcGateway(endpoint string, httpVersion string, connType clientConnType) error {
	rangeData, err := json.Marshal(&pb.RangeRequest{
		Key: []byte("a"),
	})
	if err != nil {
		return err
	}
	req := cURLReq{endpoint: "/v3/kv/range", value: string(rangeData), timeout: 5, httpVersion: httpVersion}
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
	type keyValue struct {
		Key            []byte `json:"key,omitempty"`
		CreateRevision int64  `json:"create_revision,string,omitempty"`
		ModRevision    int64  `json:"mod_revision,string,omitempty"`
		Version        int64  `json:"version,string,omitempty"`
		Value          []byte `json:"value,omitempty"`
		Lease          int64  `json:"lease,omitempty"`
	}
	type rangeResponse struct {
		Header *responseHeader `json:"header,omitempty"`
		Kvs    []*keyValue     `json:"kvs,omitempty"`
		More   bool            `json:"more,omitempty"`
		Count  int64           `json:"count,string,omitempty"`
	}
	var resp rangeResponse
	return json.Unmarshal(respData, &resp)
}

func fetchMetrics(endpoint string, httpVersion string, connType clientConnType) error {
	req := cURLReq{endpoint: "/metrics", timeout: 5, httpVersion: httpVersion}
	respData, err := curl(endpoint, "GET", req, connType)
	if err != nil {
		return err
	}
	var parser expfmt.TextParser
	_, err = parser.TextToMetricFamilies(strings.NewReader(strings.ReplaceAll(respData, "\r\n", "\n")))
	return err
}

func fetchVersion(endpoint string, httpVersion string, connType clientConnType) error {
	req := cURLReq{endpoint: "/version", timeout: 5, httpVersion: httpVersion}
	respData, err := curl(endpoint, "GET", req, connType)
	if err != nil {
		return err
	}
	var resp version.Versions
	return json.Unmarshal([]byte(respData), &resp)
}

func fetchHealth(endpoint string, httpVersion string, connType clientConnType) error {
	req := cURLReq{endpoint: "/health", timeout: 5, httpVersion: httpVersion}
	respData, err := curl(endpoint, "GET", req, connType)
	if err != nil {
		return err
	}
	var resp etcdhttp.Health
	return json.Unmarshal([]byte(respData), &resp)
}

func fetchDebugVars(endpoint string, httpVersion string, connType clientConnType) error {
	req := cURLReq{endpoint: "/debug/vars", timeout: 5, httpVersion: httpVersion}
	respData, err := curl(endpoint, "GET", req, connType)
	if err != nil {
		return err
	}
	var resp map[string]interface{}
	return json.Unmarshal([]byte(respData), &resp)
}

func curl(endpoint string, method string, curlReq cURLReq, connType clientConnType) (string, error) {
	args := cURLPrefixArgs(endpoint, connType, false, method, curlReq)
	lines, err := runUtilCompletion(args, nil)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}
