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
	"testing"
	"time"

	"go.etcd.io/etcd/v3/etcdserver/api/v3rpc/rpctypes"
	"go.etcd.io/etcd/v3/pkg/testutil"

	"google.golang.org/grpc"
)

func TestDialCancel(t *testing.T) {
	defer testutil.AfterTest(t)

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
	c, err := New(cfg)
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
	defer testutil.AfterTest(t)

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
			c, err := New(cfg)
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
			if err != context.DeadlineExceeded {
				t.Errorf("#%d: unexpected error %v, want %v", i, err, context.DeadlineExceeded)
			}
		}
	}
}

func TestDialNoTimeout(t *testing.T) {
	cfg := Config{Endpoints: []string{"127.0.0.1:12345"}}
	c, err := New(cfg)
	if c == nil || err != nil {
		t.Fatalf("new client with DialNoWait should succeed, got %v", err)
	}
	c.Close()
}

func TestIsHaltErr(t *testing.T) {
	if !isHaltErr(nil, fmt.Errorf("etcdserver: some etcdserver error")) {
		t.Errorf(`error prefixed with "etcdserver: " should be Halted by default`)
	}
	if isHaltErr(nil, rpctypes.ErrGRPCStopped) {
		t.Errorf("error %v should not halt", rpctypes.ErrGRPCStopped)
	}
	if isHaltErr(nil, rpctypes.ErrGRPCNoLeader) {
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
