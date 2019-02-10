// Copyright 2018 The etcd Authors
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

package balancer

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/testutil"

	"google.golang.org/grpc"
)

var endpoints = []string{"localhost:2379", "localhost:22379", "localhost:32379"}

func TestOldHealthBalancerGetUnblocking(t *testing.T) {
	hb := NewGRPC17Health(endpoints, minHealthRetryDuration, func(ep string, dopts ...grpc.DialOption) (*grpc.ClientConn, error) { return nil, nil })
	defer hb.Close()
	if addrs := <-hb.Notify(); len(addrs) != len(endpoints) {
		t.Errorf("Initialize NewGRPC17Health should have triggered Notify() chan, but it didn't")
	}
	unblockingOpts := grpc.BalancerGetOptions{BlockingWait: false}

	_, _, err := hb.Get(context.Background(), unblockingOpts)
	if err != ErrNoAddrAvilable {
		t.Errorf("Get() with no up endpoints should return ErrNoAddrAvailable, got: %v", err)
	}

	down1 := hb.Up(grpc.Address{Addr: endpoints[1]})
	if addrs := <-hb.Notify(); len(addrs) != 1 {
		t.Errorf("first Up() should have triggered balancer to send the first connected address via Notify chan so that other connections can be closed")
	}
	down2 := hb.Up(grpc.Address{Addr: endpoints[2]})
	addrFirst, putFun, err := hb.Get(context.Background(), unblockingOpts)
	if err != nil {
		t.Errorf("Get() with up endpoints should success, got %v", err)
	}
	if addrFirst.Addr != endpoints[1] {
		t.Errorf("Get() didn't return expected address, got %v", addrFirst)
	}
	if putFun == nil {
		t.Errorf("Get() returned unexpected nil put function")
	}
	addrSecond, _, _ := hb.Get(context.Background(), unblockingOpts)
	if addrFirst.Addr != addrSecond.Addr {
		t.Errorf("Get() didn't return the same address as previous call, got %v and %v", addrFirst, addrSecond)
	}

	down1(errors.New("error"))
	if addrs := <-hb.Notify(); len(addrs) != len(endpoints)-1 { // we call down on one endpoint
		t.Errorf("closing the only connection should triggered balancer to send the %d endpoints via Notify chan so that we can establish a connection", len(endpoints)-1)
	}
	down2(errors.New("error"))
	_, _, err = hb.Get(context.Background(), unblockingOpts)
	if err != ErrNoAddrAvilable {
		t.Errorf("Get() with no up endpoints should return ErrNoAddrAvailable, got: %v", err)
	}
}

func TestOldHealthBalancerGetBlocking(t *testing.T) {
	hb := NewGRPC17Health(endpoints, minHealthRetryDuration, func(ep string, dopts ...grpc.DialOption) (*grpc.ClientConn, error) { return nil, nil })
	defer hb.Close()
	if addrs := <-hb.Notify(); len(addrs) != len(endpoints) {
		t.Errorf("Initialize NewGRPC17Health should have triggered Notify() chan, but it didn't")
	}
	blockingOpts := grpc.BalancerGetOptions{BlockingWait: true}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	_, _, err := hb.Get(ctx, blockingOpts)
	cancel()
	if err != context.DeadlineExceeded {
		t.Errorf("Get() with no up endpoints should timeout, got %v", err)
	}

	downC := make(chan func(error), 1)

	go func() {
		// ensure hb.Up() will be called after hb.Get() to see if Up() releases blocking Get()
		time.Sleep(time.Millisecond * 100)
		f := hb.Up(grpc.Address{Addr: endpoints[1]})
		if addrs := <-hb.Notify(); len(addrs) != 1 {
			t.Errorf("first Up() should have triggered balancer to send the first connected address via Notify chan so that other connections can be closed")
		}
		downC <- f
	}()
	addrFirst, putFun, err := hb.Get(context.Background(), blockingOpts)
	if err != nil {
		t.Errorf("Get() with up endpoints should success, got %v", err)
	}
	if addrFirst.Addr != endpoints[1] {
		t.Errorf("Get() didn't return expected address, got %v", addrFirst)
	}
	if putFun == nil {
		t.Errorf("Get() returned unexpected nil put function")
	}
	down1 := <-downC

	down2 := hb.Up(grpc.Address{Addr: endpoints[2]})
	addrSecond, _, _ := hb.Get(context.Background(), blockingOpts)
	if addrFirst.Addr != addrSecond.Addr {
		t.Errorf("Get() didn't return the same address as previous call, got %v and %v", addrFirst, addrSecond)
	}

	down1(errors.New("error"))
	if addrs := <-hb.Notify(); len(addrs) != len(endpoints)-1 { // we call down on one endpoint
		t.Errorf("closing the only connection should triggered balancer to send the %d endpoints via Notify chan so that we can establish a connection", len(endpoints)-1)
	}
	down2(errors.New("error"))
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*100)
	_, _, err = hb.Get(ctx, blockingOpts)
	cancel()
	if err != context.DeadlineExceeded {
		t.Errorf("Get() with no up endpoints should timeout, got %v", err)
	}
}

// TestOldHealthBalancerGraylist checks one endpoint is tried after the other
// due to gray listing.
func TestOldHealthBalancerGraylist(t *testing.T) {
	var wg sync.WaitGroup
	// Use 3 endpoints so gray list doesn't fallback to all connections
	// after failing on 2 endpoints.
	lns, eps := make([]net.Listener, 3), make([]string, 3)
	wg.Add(3)
	connc := make(chan string, 2)
	for i := range eps {
		ln, err := net.Listen("tcp", ":0")
		testutil.AssertNil(t, err)
		lns[i], eps[i] = ln, ln.Addr().String()
		go func() {
			defer wg.Done()
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				_, err = conn.Read(make([]byte, 512))
				conn.Close()
				if err == nil {
					select {
					case connc <- ln.Addr().String():
						// sleep some so balancer catches up
						// before attempted next reconnect.
						time.Sleep(50 * time.Millisecond)
					default:
					}
				}
			}
		}()
	}

	hb := NewGRPC17Health(eps, 5*time.Second, func(ep string, dopts ...grpc.DialOption) (*grpc.ClientConn, error) { return nil, nil })

	conn, err := grpc.Dial("", grpc.WithInsecure(), grpc.WithBalancer(hb))
	testutil.AssertNil(t, err)
	defer conn.Close()

	kvc := pb.NewKVClient(conn)
	<-hb.Ready()

	kvc.Range(context.TODO(), &pb.RangeRequest{})
	ep1 := <-connc
	kvc.Range(context.TODO(), &pb.RangeRequest{})
	ep2 := <-connc
	for _, ln := range lns {
		ln.Close()
	}
	wg.Wait()

	if ep1 == ep2 {
		t.Fatalf("expected %q != %q", ep1, ep2)
	}
}

// TestBalancerDoNotBlockOnClose ensures that balancer and grpc don't deadlock each other
// due to rapid open/close conn. The deadlock causes balancer.Close() to block forever.
// See issue: https://github.com/etcd-io/etcd/issues/7283 for more detail.
func TestOldHealthBalancerDoNotBlockOnClose(t *testing.T) {
	defer testutil.AfterTest(t)

	kcl := newKillConnListener(t, 3)
	defer kcl.close()

	for i := 0; i < 5; i++ {
		hb := NewGRPC17Health(kcl.endpoints(), minHealthRetryDuration, func(ep string, dopts ...grpc.DialOption) (*grpc.ClientConn, error) { return nil, nil })
		conn, err := grpc.Dial("", grpc.WithInsecure(), grpc.WithBalancer(hb))
		if err != nil {
			t.Fatal(err)
		}
		kvc := pb.NewKVClient(conn)
		<-hb.readyc

		var wg sync.WaitGroup
		wg.Add(100)
		cctx, cancel := context.WithCancel(context.TODO())
		for j := 0; j < 100; j++ {
			go func() {
				defer wg.Done()
				kvc.Range(cctx, &pb.RangeRequest{}, grpc.FailFast(false))
			}()
		}
		// balancer.Close() might block
		// if balancer and grpc deadlock each other.
		bclosec, cclosec := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(bclosec)
			hb.Close()
		}()
		go func() {
			defer close(cclosec)
			conn.Close()
		}()
		select {
		case <-bclosec:
		case <-time.After(3 * time.Second):
			testutil.FatalStack(t, "balancer close timeout")
		}
		select {
		case <-cclosec:
		case <-time.After(3 * time.Second):
			t.Fatal("grpc conn close timeout")
		}

		cancel()
		wg.Wait()
	}
}

// killConnListener listens incoming conn and kills it immediately.
type killConnListener struct {
	wg    sync.WaitGroup
	eps   []string
	stopc chan struct{}
	t     *testing.T
}

func newKillConnListener(t *testing.T, size int) *killConnListener {
	kcl := &killConnListener{stopc: make(chan struct{}), t: t}

	for i := 0; i < size; i++ {
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		kcl.eps = append(kcl.eps, ln.Addr().String())
		kcl.wg.Add(1)
		go kcl.listen(ln)
	}
	return kcl
}

func (kcl *killConnListener) endpoints() []string {
	return kcl.eps
}

func (kcl *killConnListener) listen(l net.Listener) {
	go func() {
		defer kcl.wg.Done()
		for {
			conn, err := l.Accept()
			select {
			case <-kcl.stopc:
				return
			default:
			}
			if err != nil {
				kcl.t.Fatal(err)
			}
			time.Sleep(1 * time.Millisecond)
			conn.Close()
		}
	}()
	<-kcl.stopc
	l.Close()
}

func (kcl *killConnListener) close() {
	close(kcl.stopc)
	kcl.wg.Wait()
}
