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
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/coreos/etcd/etcdserver/stats"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestHTTPStatsAPILeaderSuccess(t *testing.T) {
	followers := make(map[string]*stats.FollowerStats)
	followers["6e3bd23ae5f1eae0"] = &stats.FollowerStats{
		Latency: stats.LatencyStats{
			Average:           0.017039507382550306,
			Current:           0.000138,
			Maximum:           1.007649,
			Minimum:           0,
			StandardDeviation: 0.05289178277920594,
		},
		Counts: stats.CountsStats{
			Fail:    0,
			Success: 745,
		},
	}
	followers["a8266ecf031671f3"] = &stats.FollowerStats{
		Latency: stats.LatencyStats{
			Average:           0.012124141496598642,
			Current:           0.000559,
			Maximum:           0.791547,
			Minimum:           0,
			StandardDeviation: 0.04187900156583733,
		},
		Counts: stats.CountsStats{
			Fail:    0,
			Success: 735,
		},
	}

	wantResponseStats := stats.LeaderStats{
		Leader:    "924e2e83e93f2560",
		Followers: followers,
	}

	jsonStats, err := json.Marshal(wantResponseStats)
	if err != nil {
		t.Errorf("got non-nil err: %#v", err)
	}

	wantAction := &statsAPIActionLeader{}
	sAPI := &httpStatsAPI{
		client: &actionAssertingHTTPClient{
			t:   t,
			act: wantAction,
			resp: http.Response{
				StatusCode: http.StatusOK,
			},
			body: jsonStats,
		},
	}

	s, err := sAPI.Leader(context.Background())
	if err != nil {
		t.Errorf("got non-nil err: %#v", err)
	}

	got, err := json.Marshal(s)
	if err != nil {
		t.Errorf("got non-nil err: %#v", err)
	}

	if !reflect.DeepEqual(jsonStats, got) {
		t.Errorf("incorrect leader stats: want=%#v got=%#v", jsonStats, got)
	}
}

func TestHTTPStatsAPILeaderError(t *testing.T) {
	tests := []httpClient{
		// generic httpClient failure
		&staticHTTPClient{err: errors.New("fail!")},

		// unrecognized HTTP status code
		&staticHTTPClient{
			resp: http.Response{StatusCode: http.StatusTeapot},
		},

		// fail to unmarshal body on StatusOK
		&staticHTTPClient{
			resp: http.Response{
				StatusCode: http.StatusOK,
			},
			body: []byte(`[{"id":"XX`),
		},
	}

	for i, tt := range tests {
		sAPI := &httpStatsAPI{client: tt}
		s, err := sAPI.Leader(context.Background())
		if err == nil {
			t.Errorf("#%d: got nil err", i)
		}
		if s != nil {
			t.Errorf("#%d: got non-nil leader stats", i)
		}
	}
}
