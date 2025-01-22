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

package etcdhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/raft/v3"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/config"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

type fakeHealthServer struct {
	fakeServer
	serializableReadError error
	linearizableReadError error
	missingLeader         bool
	authStore             auth.AuthStore
	isLearner             bool
}

func (s *fakeHealthServer) Range(_ context.Context, req *pb.RangeRequest) (*pb.RangeResponse, error) {
	if req.Serializable {
		return nil, s.serializableReadError
	}
	return nil, s.linearizableReadError
}

func (s *fakeHealthServer) IsLearner() bool {
	return s.isLearner
}

func (s *fakeHealthServer) Config() config.ServerConfig {
	return config.ServerConfig{}
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
	isLearner     bool
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
			be, _ := betesting.NewDefaultTmpBackend(t)
			defer betesting.Close(t, be)
			HandleHealth(zaptest.NewLogger(t), mux, &fakeHealthServer{
				fakeServer:            fakeServer{alarms: tt.alarms},
				serializableReadError: tt.apiError,
				linearizableReadError: tt.apiError,
				missingLeader:         tt.missingLeader,
				authStore:             auth.NewAuthStore(lg, schema.NewAuthBackend(lg, be), nil, 0),
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHTTPResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, nil, nil)
		})
	}
}

func TestHTTPSubPath(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)
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
			name:             "/readyz/learner ok",
			healthCheckURL:   "/readyz/non_learner",
			expectStatusCode: http.StatusOK,
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
				serializableReadError: tt.apiError,
				authStore:             auth.NewAuthStore(logger, schema.NewAuthBackend(logger, be), nil, 0),
			}
			HandleHealth(logger, mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHTTPResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
			checkMetrics(t, tt.healthCheckURL, "", tt.expectStatusCode)
		})
	}
}

func TestDataCorruptionCheck(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)
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
				authStore: auth.NewAuthStore(logger, schema.NewAuthBackend(logger, be), nil, 0),
			}
			HandleHealth(logger, mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			// OK before alarms are activated.
			checkHTTPResponse(t, ts, tt.healthCheckURL, http.StatusOK, nil, nil)
			// Activate the alarms.
			s.alarms = tt.alarms
			checkHTTPResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
		})
	}
}

func TestSerializableReadCheck(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)
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
			inResult:         []string{"[-]serializable_read failed: Unexpected error"},
		},
		{
			name:             "Not ready if range api is not available",
			healthCheckURL:   "/readyz",
			apiError:         fmt.Errorf("Unexpected error"),
			expectStatusCode: http.StatusServiceUnavailable,
			inResult:         []string{"[-]serializable_read failed: Unexpected error"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			logger := zaptest.NewLogger(t)
			s := &fakeHealthServer{
				serializableReadError: tt.apiError,
				authStore:             auth.NewAuthStore(logger, schema.NewAuthBackend(logger, be), nil, 0),
			}
			HandleHealth(logger, mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHTTPResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
			checkMetrics(t, tt.healthCheckURL, "serializable_read", tt.expectStatusCode)
		})
	}
}

func TestLinearizableReadCheck(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)
	tests := []healthTestCase{
		{
			name:             "Alive normal",
			healthCheckURL:   "/livez?verbose",
			expectStatusCode: http.StatusOK,
			inResult:         []string{"[+]serializable_read ok"},
		},
		{
			name:             "Alive if lineariable range api is not available",
			healthCheckURL:   "/livez",
			apiError:         fmt.Errorf("Unexpected error"),
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Not ready if range api is not available",
			healthCheckURL:   "/readyz",
			apiError:         fmt.Errorf("Unexpected error"),
			expectStatusCode: http.StatusServiceUnavailable,
			inResult:         []string{"[+]serializable_read ok", "[-]linearizable_read failed: Unexpected error"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			logger := zaptest.NewLogger(t)
			s := &fakeHealthServer{
				linearizableReadError: tt.apiError,
				authStore:             auth.NewAuthStore(logger, schema.NewAuthBackend(logger, be), nil, 0),
			}
			HandleHealth(logger, mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHTTPResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
			checkMetrics(t, tt.healthCheckURL, "linearizable_read", tt.expectStatusCode)
		})
	}
}

func TestLearnerReadyCheck(t *testing.T) {
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)
	tests := []healthTestCase{
		{
			name:             "readyz normal",
			healthCheckURL:   "/readyz",
			expectStatusCode: http.StatusOK,
			isLearner:        false,
		},
		{
			name:             "not ready because member is learner",
			healthCheckURL:   "/readyz",
			expectStatusCode: http.StatusServiceUnavailable,
			isLearner:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			logger := zaptest.NewLogger(t)
			s := &fakeHealthServer{
				linearizableReadError: tt.apiError,
				authStore:             auth.NewAuthStore(logger, schema.NewAuthBackend(logger, be), nil, 0),
			}
			s.isLearner = tt.isLearner
			HandleHealth(logger, mux, s)
			ts := httptest.NewServer(mux)
			defer ts.Close()
			checkHTTPResponse(t, ts, tt.healthCheckURL, tt.expectStatusCode, tt.inResult, tt.notInResult)
			checkMetrics(t, tt.healthCheckURL, "linearizable_read", tt.expectStatusCode)
		})
	}
}

func checkHTTPResponse(t *testing.T, ts *httptest.Server, url string, expectStatusCode int, inResult []string, notInResult []string) {
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

func checkMetrics(t *testing.T, url, checkName string, expectStatusCode int) {
	defer healthCheckGauge.Reset()
	defer healthCheckCounter.Reset()

	typeName := strings.TrimPrefix(strings.Split(url, "?")[0], "/")
	if len(checkName) == 0 {
		checkName = strings.Split(typeName, "/")[1]
		typeName = strings.Split(typeName, "/")[0]
	}

	expectedSuccessCount := 1
	expectedErrorCount := 0
	if expectStatusCode != http.StatusOK {
		expectedSuccessCount = 0
		expectedErrorCount = 1
	}

	gather, _ := prometheus.DefaultGatherer.Gather()
	for _, mf := range gather {
		name := *mf.Name
		val := 0
		switch name {
		case "etcd_server_healthcheck":
			val = int(mf.GetMetric()[0].GetGauge().GetValue())
		case "etcd_server_healthcheck_total":
			val = int(mf.GetMetric()[0].GetCounter().GetValue())
		default:
			continue
		}
		labelMap := make(map[string]string)
		for _, label := range mf.GetMetric()[0].Label {
			labelMap[label.GetName()] = label.GetValue()
		}
		if typeName != labelMap["type"] {
			continue
		}
		if labelMap["name"] != checkName {
			continue
		}
		if statusLabel, found := labelMap["status"]; found && statusLabel == HealthStatusError {
			if val != expectedErrorCount {
				t.Fatalf("%s got errorCount %d, wanted %d\n", name, val, expectedErrorCount)
			}
		} else {
			if val != expectedSuccessCount {
				t.Fatalf("%s got expectedSuccessCount %d, wanted %d\n", name, val, expectedSuccessCount)
			}
		}
	}
}
