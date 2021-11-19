// Copyright 2021 The etcd Authors
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

// Package version implements etcd version parsing and contains latest version
// information.

package etcdserver

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.uber.org/zap"
)

func TestBootstrapExistingClusterNoWALMaxLearner(t *testing.T) {
	tests := []struct {
		name          string
		members       []etcdserverpb.Member
		maxLearner    int
		hasError      bool
		expectedError error
	}{
		{
			name: "bootstrap success: maxLearner gt learner count",
			members: []etcdserverpb.Member{
				{ID: 4512484362714696085, PeerURLs: []string{"http://localhost:2380"}},
				{ID: 5321713336100798248, PeerURLs: []string{"http://localhost:2381"}},
				{ID: 5670219998796287055, PeerURLs: []string{"http://localhost:2382"}},
			},
			maxLearner:    1,
			hasError:      false,
			expectedError: nil,
		},
		{
			name: "bootstrap success: maxLearner eq learner count",
			members: []etcdserverpb.Member{
				{ID: 4512484362714696085, PeerURLs: []string{"http://localhost:2380"}, IsLearner: true},
				{ID: 5321713336100798248, PeerURLs: []string{"http://localhost:2381"}},
				{ID: 5670219998796287055, PeerURLs: []string{"http://localhost:2382"}, IsLearner: true},
			},
			maxLearner:    2,
			hasError:      false,
			expectedError: nil,
		},
		{
			name: "bootstrap fail: maxLearner lt learner count",
			members: []etcdserverpb.Member{
				{ID: 4512484362714696085, PeerURLs: []string{"http://localhost:2380"}},
				{ID: 5321713336100798248, PeerURLs: []string{"http://localhost:2381"}, IsLearner: true},
				{ID: 5670219998796287055, PeerURLs: []string{"http://localhost:2382"}, IsLearner: true},
			},
			maxLearner:    1,
			hasError:      true,
			expectedError: membership.ErrTooManyLearners,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := types.NewURLsMap("node0=http://localhost:2380,node1=http://localhost:2381,node2=http://localhost:2382")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			cfg := config.ServerConfig{
				Name:                    "node0",
				InitialPeerURLsMap:      cluster,
				Logger:                  zap.NewExample(),
				ExperimentalMaxLearners: tt.maxLearner,
			}
			_, err = bootstrapExistingClusterNoWAL(cfg, mockBootstrapRoundTrip(tt.members))
			hasError := err != nil
			if hasError != tt.hasError {
				t.Errorf("expected error: %v got: %v", tt.hasError, err)
			}
			if hasError && !strings.Contains(err.Error(), tt.expectedError.Error()) {
				t.Fatalf("expected error to contain: %q, got: %q", tt.expectedError.Error(), err.Error())
			}
		})
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (s roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return s(r)
}

func mockBootstrapRoundTrip(members []etcdserverpb.Member) roundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(r.URL.String(), "/members"):
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(mockMembersJSON(members))),
				Header:     http.Header{"X-Etcd-Cluster-Id": []string{"f4588138892a16b0"}},
			}, nil
		case strings.Contains(r.URL.String(), "/version"):
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(mockVersionJSON())),
			}, nil
		case strings.Contains(r.URL.String(), DowngradeEnabledPath):
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`true`)),
			}, nil
		}
		return nil, nil
	}
}

func mockVersionJSON() string {
	v := version.Versions{Server: "3.7.0", Cluster: "3.7.0"}
	version, _ := json.Marshal(v)
	return string(version)
}

func mockMembersJSON(m []etcdserverpb.Member) string {
	members, _ := json.Marshal(m)
	return string(members)
}
