package etcdhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	stats "go.etcd.io/etcd/server/v3/etcdserver/api/v2stats"
	"go.uber.org/zap"
)

type fakeStats struct{}

func (s *fakeStats) SelfStats() []byte   { return nil }
func (s *fakeStats) LeaderStats() []byte { return nil }
func (s *fakeStats) StoreStats() []byte  { return nil }

type fakeServerV2 struct {
	fakeServer
	stats.Stats
	health string
}

func (s *fakeServerV2) Leader() types.ID {
	if s.health == "true" {
		return 1
	}
	return types.ID(raft.None)
}
func (s *fakeServerV2) Do(ctx context.Context, r pb.Request) (etcdserver.Response, error) {
	if s.health == "true" {
		return etcdserver.Response{}, nil
	}
	return etcdserver.Response{}, fmt.Errorf("fail health check")
}
func (s *fakeServerV2) ClientCertAuthEnabled() bool { return false }

func TestHealthHandler(t *testing.T) {
	// define the input and expected output
	// input: alarms, and healthCheckURL
	tests := []struct {
		name           string
		alarms         []*pb.AlarmMember
		healthCheckURL string

		expectStatusCode int
		expectHealth     string
	}{
		{
			name:             "Healthy if no alarm",
			alarms:           []*pb.AlarmMember{},
			healthCheckURL:   "/health",
			expectStatusCode: http.StatusOK,
			expectHealth:     "true",
		},
		{
			name:             "Unhealthy if NOSPACE alarm is on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/health",
			expectStatusCode: http.StatusServiceUnavailable,
			expectHealth:     "false",
		},
		{
			name:             "Healthy if NOSPACE alarm is on and excluded",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusOK,
			expectHealth:     "true",
		},
		{
			name:             "Healthy if NOSPACE alarm is excluded",
			alarms:           []*pb.AlarmMember{},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusOK,
			expectHealth:     "true",
		},
		{
			name:             "Healthy if multiple NOSPACE alarms are on and excluded",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(1), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(2), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(3), Alarm: pb.AlarmType_NOSPACE}},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusOK,
			expectHealth:     "true",
		},
		{
			name:             "Unhealthy if NOSPACE alarms is excluded and CORRUPT is on",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(1), Alarm: pb.AlarmType_CORRUPT}},
			healthCheckURL:   "/health?exclude=NOSPACE",
			expectStatusCode: http.StatusServiceUnavailable,
			expectHealth:     "false",
		},
		{
			name:             "Unhealthy if both NOSPACE and CORRUPT are on and excluded",
			alarms:           []*pb.AlarmMember{{MemberID: uint64(0), Alarm: pb.AlarmType_NOSPACE}, {MemberID: uint64(1), Alarm: pb.AlarmType_CORRUPT}},
			healthCheckURL:   "/health?exclude=NOSPACE&exclude=CORRUPT",
			expectStatusCode: http.StatusOK,
			expectHealth:     "true",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			HandleMetricsHealth(zap.NewExample(), mux, &fakeServerV2{
				fakeServer: fakeServer{alarms: tt.alarms},
				Stats:      &fakeStats{},
				health:     tt.expectHealth,
			})
			ts := httptest.NewServer(mux)
			defer ts.Close()

			res, err := ts.Client().Do(&http.Request{Method: http.MethodGet, URL: testutil.MustNewURL(t, ts.URL+tt.healthCheckURL)})
			if err != nil {
				t.Errorf("fail serve http request %s %v in test case #%d", tt.healthCheckURL, err, i+1)
			}
			if res == nil {
				t.Errorf("got nil http response with http request %s in test case #%d", tt.healthCheckURL, i+1)
				return
			}
			if res.StatusCode != tt.expectStatusCode {
				t.Errorf("want statusCode %d but got %d in test case #%d", tt.expectStatusCode, res.StatusCode, i+1)
			}
			health, err := parseHealthOutput(res.Body)
			if err != nil {
				t.Errorf("fail parse health check output %v", err)
			}
			if health.Health != tt.expectHealth {
				t.Errorf("want health %s but got %s", tt.expectHealth, health.Health)
			}
		})
	}
}

func parseHealthOutput(body io.Reader) (Health, error) {
	obj := Health{}
	d, derr := ioutil.ReadAll(body)
	if derr != nil {
		return obj, derr
	}
	if err := json.Unmarshal(d, &obj); err != nil {
		return obj, err
	}
	return obj, nil
}
