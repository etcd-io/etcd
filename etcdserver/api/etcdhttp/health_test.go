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

package etcdhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/auth"
	"go.etcd.io/etcd/etcdserver"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	betesting "go.etcd.io/etcd/mvcc/backend"
	"go.etcd.io/etcd/pkg/testutil"
	"go.etcd.io/etcd/pkg/types"
	"go.etcd.io/etcd/raft"
)

type fakeHealthServer struct {
	fakeServer
	apiError      error
	missingLeader bool
	authStore     auth.AuthStore
}

func (s *fakeHealthServer) Range(_ context.Context, _ *pb.RangeRequest) (*pb.RangeResponse, error) {
	return nil, s.apiError
}

func (s *fakeHealthServer) Config() etcdserver.ServerConfig {
	return etcdserver.ServerConfig{}
}

func (s *fakeHealthServer) Leader() types.ID {
	if !s.missingLeader {
		return 1
	}
	return types.ID(raft.None)
}

func (s *fakeHealthServer) AuthStore() auth.AuthStore { return s.authStore }

func (s *fakeHealthServer) ClientCertAuthEnabled() bool { return false }

type healthTestCase struct {
	name             string
	healthCheckURL   string
	expectStatusCode int
	inResult         []string
	notInResult      []string

	alarms        []*pb.AlarmMember
	apiError      error
	missingLeader bool
}

func TestHealthHandler(t *testing.T) {
	// define the input and expected output
	// input: alarms, and healthCheckURL
	tests := []healthTestCase{
		{
			name:             "Healthy if no alarm",
			alarms:           []*pb.AlarmMember{},
			healthCheckURL:   "/health",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Unhealthy if NOSPACE alarm is on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/health",
			expectStatusCode: http.StatusServiceUnavailable,
		},
		{
			name:             "Healthy if NOSPACE alarm is on and excluded",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Healthy if NOSPACE alarm is excluded",
			alarms:           []*pb.AlarmMember{},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Healthy if multiple NOSPACE alarms are on and excluded",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(1), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(2), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(3), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Unhealthy if NOSPACE alarms is excluded and CORRUPT is on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(1), Alarm: pb.AlarmType_CORRUPT}},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusServiceUnavailable,
		},
		{
			name:             "Unhealthy if both NOSPACE and CORRUPT are on and excluded",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(1), Alarm: pb.AlarmType_CORRUPT}},
			healthCheckURL:   "/health?exclude=NOSPACE&exclude=CORRUPT",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Unhealthy if api is not available",
			healthCheckURL:   "/health",
			apiError:         fmt.Errorf("Unexpected error"),
			expectStatusCode: http.StatusServiceUnavailable,
		},
		{
			name:             "Unhealthy if no leader",
			healthCheckURL:   "/health",
			expectStatusCode: http.StatusServiceUnavailable,
			missingLeader:    true,
		},
		{
			name:             "Healthy if no leader and serializable=true",
			healthCheckURL:   "/health?serializable=true",
			expectStatusCode: http.StatusOK,
			missingLeader:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			lg := zaptest.NewLogger(t)
			be, _ := betesting.NewDefaultTmpBackend()
			defer be.Close()
			HandleHealth(mux, &fakeHealthServer{
				fakeServer:    fakeServer{alarms: tt.alarms},
				apiError:      tt.apiError,
				missingLeader: tt.missingLeader,
				authStore:     auth.NewAuthStore(lg, be, nil, 0),
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHttpResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, nil, nil)
		})
	}
}

func TestHttpSubPath(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend()
	defer be.Close()
	tests := []healthTestCase{
		{
			name:             "/readyz/data_corruption ok",
			healthCheckURL:   "/readyz/data_corruption",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "/readyz/serializable_read not ok with error",
			apiError:         fmt.Errorf("Unexpected error"),
			healthCheckURL:   "/readyz/serializable_read",
			expectStatusCode: http.StatusServiceUnavailable,
			notInResult:      []string{"data_corruption"},
		},
		{
			name:             "/readyz/non_exist 404",
			healthCheckURL:   "/readyz/non_exist",
			expectStatusCode: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			logger := zaptest.NewLogger(t)
			s := &fakeHealthServer{
				apiError:  tt.apiError,
				authStore: auth.NewAuthStore(logger, be, nil, 0),
			}
			HandleHealth(mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHttpResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
		})
	}
}

func TestDataCorruptionCheck(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend()
	defer be.Close()
	tests := []healthTestCase{
		{
			name:             "Live if CORRUPT alarm is on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_CORRUPT}},
			healthCheckURL:   "/livez",
			expectStatusCode: http.StatusOK,
			notInResult:      []string{"data_corruption"},
		},
		{
			name:             "Not ready if CORRUPT alarm is on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_CORRUPT}},
			healthCheckURL:   "/readyz",
			expectStatusCode: http.StatusServiceUnavailable,
			inResult:         []string{"[-]data_corruption failed: alarm activated: CORRUPT"},
		},
		{
			name:             "ready if CORRUPT alarm is not on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/readyz",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "ready if CORRUPT alarm is excluded",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_CORRUPT}, {MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/readyz?exclude=data_corruption",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Not ready if CORRUPT alarm is on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_CORRUPT}},
			healthCheckURL:   "/readyz?exclude=non_exist",
			expectStatusCode: http.StatusServiceUnavailable,
			inResult:         []string{"[-]data_corruption failed: alarm activated: CORRUPT"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			logger := zaptest.NewLogger(t)
			s := &fakeHealthServer{
				authStore: auth.NewAuthStore(logger, be, nil, 0),
			}
			HandleHealth(mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			// OK before alarms are activated.
			checkHttpResponse(t, ts, tt.healthCheckURL, http.StatusOK, nil, nil)
			// Activate the alarms.
			s.alarms = tt.alarms
			checkHttpResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
		})
	}
}

func TestSerializableReadCheck(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend()
	defer be.Close()
	tests := []healthTestCase{
		{
			name:             "Alive normal",
			healthCheckURL:   "/livez?verbose",
			expectStatusCode: http.StatusOK,
			inResult:         []string{"[+]serializable_read ok"},
		},
		{
			name:             "Not alive if range api is not available",
			healthCheckURL:   "/livez",
			apiError:         fmt.Errorf("Unexpected error"),
			expectStatusCode: http.StatusServiceUnavailable,
			inResult:         []string{"[-]serializable_read failed: range error: Unexpected error"},
		},
		{
			name:             "Not ready if range api is not available",
			healthCheckURL:   "/readyz",
			apiError:         fmt.Errorf("Unexpected error"),
			expectStatusCode: http.StatusServiceUnavailable,
			inResult:         []string{"[-]serializable_read failed: range error: Unexpected error"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			logger := zaptest.NewLogger(t)
			s := &fakeHealthServer{
				apiError:  tt.apiError,
				authStore: auth.NewAuthStore(logger, be, nil, 0),
			}
			HandleHealth(mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHttpResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
		})
	}
}

func checkHttpResponse(t *testing.T, ts *httptest.Server, url string, expectStatusCode int, inResult []string, notInResult []string) {
	res, err := ts.Client().Do(&http.Request{Method: http.MethodGet, URL: testutil.MustNewURL(t, ts.URL+url)})

	if err != nil {
		t.Fatalf("fail serve http request %s %v", url, err)
	}
	if res.StatusCode != expectStatusCode {
		t.Errorf("want statusCode %d but got %d", expectStatusCode, res.StatusCode)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("Failed to read response for %s", url)
	}
	result := string(b)
	for _, substr := range inResult {
		if !strings.Contains(result, substr) {
			t.Errorf("Could not find substring : %s, in response: %s", substr, result)
			return
		}
	}
	for _, substr := range notInResult {
		if strings.Contains(result, substr) {
			t.Errorf("Do not expect substring : %s, in response: %s", substr, result)
			return
		}
	}
}
