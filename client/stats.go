// Copyright 2015 CoreOS, Inc.
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

package client

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"

	"github.com/coreos/etcd/etcdserver/stats"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

var (
	defaultV2StatsPrefix = "/v2/stats"
)

// NewStatsAPI constructs a new StatsAPI that uses HTTP to interact with
// etcd's stats API.
func NewStatsAPI(c Client) StatsAPI {
	return &httpStatsAPI{
		client: c,
	}
}

type StatsAPI interface {
	// Leader returns the etcd leader stats.
	Leader(ctx context.Context) (*stats.LeaderStats, error)
}

type httpStatsAPI struct {
	client httpClient
}

func (m *httpStatsAPI) Leader(ctx context.Context) (*stats.LeaderStats, error) {
	req := &statsAPIActionLeader{}
	resp, body, err := m.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := assertStatusCode(resp.StatusCode, http.StatusOK); err != nil {
		return nil, err
	}

	var leaderStats stats.LeaderStats
	if err := json.Unmarshal(body, &leaderStats); err != nil {
		return nil, err
	}

	return &leaderStats, nil
}

type statsAPIActionLeader struct{}

func (l *statsAPIActionLeader) HTTPRequest(ep url.URL) *http.Request {
	u := v2StatsURL(ep)
	u.Path = path.Join(u.Path, "leader")
	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

// v2StatsURL add the necessary path to the provided endpoint
// to route requests to the default v2 stats API.
func v2StatsURL(ep url.URL) *url.URL {
	ep.Path = path.Join(ep.Path, defaultV2StatsPrefix)
	return &ep
}
