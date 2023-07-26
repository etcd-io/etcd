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

package clientv3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/testutil"

	"google.golang.org/grpc"
)

func NewClient(t *testing.T, cfg Config) (*Client, error) {
	if cfg.Logger == nil {
		cfg.Logger = zaptest.NewLogger(t).Named("client")
	}
	return New(cfg)
}

func TestDialCancel(t *testing.T) {
	testutil.RegisterLeakDetection(t)

	// accept first connection so client is created with dial timeout
	ln, err := net.Listen("unix", "dialcancel:12345")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ep := "unix://dialcancel:12345"
	cfg := Config{
		Endpoints:   []string{ep},
		DialTimeout: 30 * time.Second}
	c, err := NewClient(t, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// connect to ipv4 black hole so dial blocks
	c.SetEndpoints("http://254.0.0.1:12345")

	// issue Get to force redial attempts
	getc := make(chan struct{})
	go func() {
		defer close(getc)
		// Get may hang forever on grpc's Stream.Header() if its
		// context is never canceled.
		c.Get(c.Ctx(), "abc")
	}()

	// wait a little bit so client close is after dial starts
	time.Sleep(100 * time.Millisecond)

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		c.Close()
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("failed to close")
	case <-donec:
	}
	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("get failed to exit")
	case <-getc:
	}
}

func TestDialTimeout(t *testing.T) {
	testutil.RegisterLeakDetection(t)

	wantError := context.DeadlineExceeded

	// grpc.WithBlock to block until connection up or timeout
	testCfgs := []Config{
		{
			Endpoints:   []string{"http://254.0.0.1:12345"},
			DialTimeout: 2 * time.Second,
			DialOptions: []grpc.DialOption{grpc.WithBlock()},
		},
		{
			Endpoints:   []string{"http://254.0.0.1:12345"},
			DialTimeout: time.Second,
			DialOptions: []grpc.DialOption{grpc.WithBlock()},
			Username:    "abc",
			Password:    "def",
		},
	}

	for i, cfg := range testCfgs {
		donec := make(chan error, 1)
		go func(cfg Config, i int) {
			// without timeout, dial continues forever on ipv4 black hole
			c, err := NewClient(t, cfg)
			if c != nil || err == nil {
				t.Errorf("#%d: new client should fail", i)
			}
			donec <- err
		}(cfg, i)

		time.Sleep(10 * time.Millisecond)

		select {
		case err := <-donec:
			t.Errorf("#%d: dial didn't wait (%v)", i, err)
		default:
		}

		select {
		case <-time.After(5 * time.Second):
			t.Errorf("#%d: failed to timeout dial on time", i)
		case err := <-donec:
			if err.Error() != wantError.Error() {
				t.Errorf("#%d: unexpected error '%v', want '%v'", i, err, wantError)
			}
		}
	}
}

func TestDialNoTimeout(t *testing.T) {
	cfg := Config{Endpoints: []string{"127.0.0.1:12345"}}
	c, err := NewClient(t, cfg)
	if c == nil || err != nil {
		t.Fatalf("new client with DialNoWait should succeed, got %v", err)
	}
	c.Close()
}

func TestIsHaltErr(t *testing.T) {
	assert.Equal(t,
		isHaltErr(context.TODO(), errors.New("etcdserver: some etcdserver error")),
		true,
		"error created by errors.New should be unavailable error",
	)
	assert.Equal(t,
		isHaltErr(context.TODO(), rpctypes.ErrGRPCStopped),
		false,
		fmt.Sprintf(`error "%v" should not be halt error`, rpctypes.ErrGRPCStopped),
	)
	assert.Equal(t,
		isHaltErr(context.TODO(), rpctypes.ErrGRPCNoLeader),
		false,
		fmt.Sprintf(`error "%v" should not be halt error`, rpctypes.ErrGRPCNoLeader),
	)
	ctx, cancel := context.WithCancel(context.TODO())
	assert.Equal(t,
		isHaltErr(ctx, nil),
		false,
		"no error and active context should be halt error",
	)
	cancel()
	assert.Equal(t,
		isHaltErr(ctx, nil),
		true,
		"cancel on context should be halte error",
	)
}

func TestIsUnavailableErr(t *testing.T) {
	assert.Equal(t,
		isUnavailableErr(context.TODO(), errors.New("etcdserver: some etcdserver error")),
		false,
		"error created by errors.New should not be unavailable error",
	)
	assert.Equal(t,
		isUnavailableErr(context.TODO(), rpctypes.ErrGRPCStopped),
		true,
		fmt.Sprintf(`error "%v" should be unavailable error`, rpctypes.ErrGRPCStopped),
	)
	assert.Equal(t,
		isUnavailableErr(context.TODO(), rpctypes.ErrGRPCNotCapable),
		false,
		fmt.Sprintf("error %v should not be unavailable error", rpctypes.ErrGRPCNotCapable),
	)
	ctx, cancel := context.WithCancel(context.TODO())
	assert.Equal(t,
		isUnavailableErr(ctx, nil),
		false,
		"no error and active context should not be unavailable error",
	)
	cancel()
	assert.Equal(t,
		isUnavailableErr(ctx, nil),
		false,
		"cancel on context should not be unavailable error",
	)
}

func TestCloseCtxClient(t *testing.T) {
	ctx := context.Background()
	c := NewCtxClient(ctx)
	err := c.Close()
	// Close returns ctx.toErr, a nil error means an open Done channel
	if err == nil {
		t.Errorf("failed to Close the client. %v", err)
	}
}

func TestWithLogger(t *testing.T) {
	ctx := context.Background()
	c := NewCtxClient(ctx)
	if c.lg == nil {
		t.Errorf("unexpected nil in *zap.Logger")
	}

	c.WithLogger(nil)
	if c.lg != nil {
		t.Errorf("WithLogger should modify *zap.Logger")
	}
}

func TestZapWithLogger(t *testing.T) {
	ctx := context.Background()
	lg := zap.NewNop()
	c := NewCtxClient(ctx, WithZapLogger(lg))

	if c.lg != lg {
		t.Errorf("WithZapLogger should modify *zap.Logger")
	}
}

func TestAuthTokenBundleNoOverwrite(t *testing.T) {
	// This call in particular changes working directory to the tmp dir of
	// the test. The `etcd-auth-test:0` can be created in local directory,
	// not exceeding the longest allowed path on OsX.
	testutil.BeforeTest(t)

	// Create a mock AuthServer to handle Authenticate RPCs.
	lis, err := net.Listen("unix", "etcd-auth-test:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	addr := "unix://" + lis.Addr().String()
	srv := grpc.NewServer()
	etcdserverpb.RegisterAuthServer(srv, mockAuthServer{})
	go srv.Serve(lis)
	defer srv.Stop()

	// Create a client, which should call Authenticate on the mock server to
	// exchange username/password for an auth token.
	c, err := NewClient(t, Config{
		DialTimeout: 5 * time.Second,
		Endpoints:   []string{addr},
		Username:    "foo",
		Password:    "bar",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	oldTokenBundle := c.authTokenBundle

	// Call the public Dial again, which should preserve the original
	// authTokenBundle.
	gc, err := c.Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer gc.Close()
	newTokenBundle := c.authTokenBundle

	if oldTokenBundle != newTokenBundle {
		t.Error("Client.authTokenBundle has been overwritten during Client.Dial")
	}
}

func TestSyncFiltersMembers(t *testing.T) {
	c, _ := NewClient(t, Config{Endpoints: []string{"http://254.0.0.1:12345"}})
	defer c.Close()
	c.Cluster = &mockCluster{
		[]*etcdserverpb.Member{
			{ID: 0, Name: "", ClientURLs: []string{"http://254.0.0.1:12345"}, IsLearner: false},
			{ID: 1, Name: "isStarted", ClientURLs: []string{"http://254.0.0.2:12345"}, IsLearner: true},
			{ID: 2, Name: "isStartedAndNotLearner", ClientURLs: []string{"http://254.0.0.3:12345"}, IsLearner: false},
		},
	}
	c.Sync(context.Background())

	endpoints := c.Endpoints()
	if len(endpoints) != 1 || endpoints[0] != "http://254.0.0.3:12345" {
		t.Error("Client.Sync uses learner and/or non-started member client URLs")
	}
}

func TestMinSupportedVersion(t *testing.T) {
	testutil.BeforeTest(t)
	var tests = []struct {
		name                string
		currentVersion      semver.Version
		minSupportedVersion semver.Version
	}{
		{
			name:                "v3.6 client should accept v3.5",
			currentVersion:      version.V3_6,
			minSupportedVersion: version.V3_5,
		},
		{
			name:                "v3.7 client should accept v3.6",
			currentVersion:      version.V3_7,
			minSupportedVersion: version.V3_6,
		},
		{
			name:                "first minor version should accept its previous version",
			currentVersion:      version.V4_0,
			minSupportedVersion: version.V3_7,
		},
		{
			name:                "first version in list should not accept previous versions",
			currentVersion:      version.V3_0,
			minSupportedVersion: version.V3_0,
		},
	}

	versionBackup := version.Version
	t.Cleanup(func() {
		version.Version = versionBackup
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version.Version = tt.currentVersion.String()
			require.True(t, minSupportedVersion().Equal(tt.minSupportedVersion))
		})
	}
}

func TestClientRejectOldCluster(t *testing.T) {
	testutil.BeforeTest(t)
	var tests = []struct {
		name          string
		endpoints     []string
		versions      []string
		expectedError error
	}{
		{
			name:          "all new versions with the same value",
			endpoints:     []string{"192.168.3.41:22379", "192.168.3.41:22479", "192.168.3.41:22579"},
			versions:      []string{version.Version, version.Version, version.Version},
			expectedError: nil,
		},
		{
			name:          "all new versions with different values",
			endpoints:     []string{"192.168.3.41:22379", "192.168.3.41:22479", "192.168.3.41:22579"},
			versions:      []string{version.Version, minSupportedVersion().String(), minSupportedVersion().String()},
			expectedError: nil,
		},
		{
			name:          "all old versions with different values",
			endpoints:     []string{"192.168.3.41:22379", "192.168.3.41:22479", "192.168.3.41:22579"},
			versions:      []string{"3.3.0", "3.3.0", "3.4.0"},
			expectedError: ErrOldCluster,
		},
		{
			name:          "all old versions with the same value",
			endpoints:     []string{"192.168.3.41:22379", "192.168.3.41:22479", "192.168.3.41:22579"},
			versions:      []string{"3.3.0", "3.3.0", "3.3.0"},
			expectedError: ErrOldCluster,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.endpoints) != len(tt.versions) || len(tt.endpoints) == 0 {
				t.Errorf("Unexpected endpoints and versions length, len(endpoints):%d, len(versions):%d", len(tt.endpoints), len(tt.versions))
				return
			}
			endpointToVersion := make(map[string]string)
			for j := range tt.endpoints {
				endpointToVersion[tt.endpoints[j]] = tt.versions[j]
			}
			c := &Client{
				ctx:       context.Background(),
				endpoints: tt.endpoints,
				epMu:      new(sync.RWMutex),
				Maintenance: &mockMaintenance{
					Version: endpointToVersion,
				},
			}

			if err := c.checkVersion(); err != tt.expectedError {
				t.Errorf("heckVersion err:%v", err)
			}
		})

	}

}

type mockMaintenance struct {
	Version map[string]string
}

func (mm mockMaintenance) Status(ctx context.Context, endpoint string) (*StatusResponse, error) {
	return &StatusResponse{Version: mm.Version[endpoint]}, nil
}

func (mm mockMaintenance) AlarmList(ctx context.Context) (*AlarmResponse, error) {
	return nil, nil
}

func (mm mockMaintenance) AlarmDisarm(ctx context.Context, m *AlarmMember) (*AlarmResponse, error) {
	return nil, nil
}

func (mm mockMaintenance) Defragment(ctx context.Context, endpoint string) (*DefragmentResponse, error) {
	return nil, nil
}

func (mm mockMaintenance) HashKV(ctx context.Context, endpoint string, rev int64) (*HashKVResponse, error) {
	return nil, nil
}

func (mm mockMaintenance) SnapshotWithVersion(ctx context.Context) (*SnapshotResponse, error) {
	return nil, nil
}

func (mm mockMaintenance) Snapshot(ctx context.Context) (io.ReadCloser, error) {
	return nil, nil
}

func (mm mockMaintenance) MoveLeader(ctx context.Context, transfereeID uint64) (*MoveLeaderResponse, error) {
	return nil, nil
}

func (mm mockMaintenance) Downgrade(ctx context.Context, action DowngradeAction, version string) (*DowngradeResponse, error) {
	return nil, nil
}

type mockAuthServer struct {
	*etcdserverpb.UnimplementedAuthServer
}

func (mockAuthServer) Authenticate(context.Context, *etcdserverpb.AuthenticateRequest) (*etcdserverpb.AuthenticateResponse, error) {
	return &etcdserverpb.AuthenticateResponse{Token: "mock-token"}, nil
}

type mockCluster struct {
	members []*etcdserverpb.Member
}

func (mc *mockCluster) MemberList(ctx context.Context, opts ...OpOption) (*MemberListResponse, error) {
	return &MemberListResponse{Members: mc.members}, nil
}

func (mc *mockCluster) MemberAdd(ctx context.Context, peerAddrs []string) (*MemberAddResponse, error) {
	return nil, nil
}

func (mc *mockCluster) MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*MemberAddResponse, error) {
	return nil, nil
}

func (mc *mockCluster) MemberRemove(ctx context.Context, id uint64) (*MemberRemoveResponse, error) {
	return nil, nil
}

func (mc *mockCluster) MemberUpdate(ctx context.Context, id uint64, peerAddrs []string) (*MemberUpdateResponse, error) {
	return nil, nil
}

func (mc *mockCluster) MemberPromote(ctx context.Context, id uint64) (*MemberPromoteResponse, error) {
	return nil, nil
}
