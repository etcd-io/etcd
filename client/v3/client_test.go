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
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

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
		go func(cfg Config) {
			// without timeout, dial continues forever on ipv4 black hole
			c, err := NewClient(t, cfg)
			if c != nil || err == nil {
				t.Errorf("#%d: new client should fail", i)
			}
			donec <- err
		}(cfg)

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
	if !isHaltErr(context.TODO(), fmt.Errorf("etcdserver: some etcdserver error")) {
		t.Errorf(`error prefixed with "etcdserver: " should be Halted by default`)
	}
	if isHaltErr(context.TODO(), rpctypes.ErrGRPCStopped) {
		t.Errorf("error %v should not halt", rpctypes.ErrGRPCStopped)
	}
	if isHaltErr(context.TODO(), rpctypes.ErrGRPCNoLeader) {
		t.Errorf("error %v should not halt", rpctypes.ErrGRPCNoLeader)
	}
	ctx, cancel := context.WithCancel(context.TODO())
	if isHaltErr(ctx, nil) {
		t.Errorf("no error and active context should not be Halted")
	}
	cancel()
	if !isHaltErr(ctx, nil) {
		t.Errorf("cancel on context should be Halted")
	}
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
	// Create a mock AuthServer to handle Authenticate RPCs.
	lis, err := net.Listen("unix", filepath.Join(t.TempDir(), "etcd-auth-test:0"))
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

type mockAuthServer struct {
	*etcdserverpb.UnimplementedAuthServer
}

func (mockAuthServer) Authenticate(context.Context, *etcdserverpb.AuthenticateRequest) (*etcdserverpb.AuthenticateResponse, error) {
	return &etcdserverpb.AuthenticateResponse{Token: "mock-token"}, nil
}
