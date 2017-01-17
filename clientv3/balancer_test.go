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
	"errors"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	endpoints = []string{"localhost:2379", "localhost:22379", "localhost:32379"}
)

func TestBalancerGetUnblocking(t *testing.T) {
	sb := newSimpleBalancer(endpoints)
	if addrs := <-sb.Notify(); len(addrs) != len(endpoints) {
		t.Errorf("Initialize newSimpleBalancer should have triggered Notify() chan, but it didn't")
	}
	unblockingOpts := grpc.BalancerGetOptions{BlockingWait: false}

	_, _, err := sb.Get(context.Background(), unblockingOpts)
	if err != ErrNoAddrAvilable {
		t.Errorf("Get() with no up endpoints should return ErrNoAddrAvailable, got: %v", err)
	}

	down1 := sb.Up(grpc.Address{Addr: endpoints[1]})
	if addrs := <-sb.Notify(); len(addrs) != 1 {
		t.Errorf("first Up() should have triggered balancer to send the first connected address via Notify chan so that other connections can be closed")
	}
	down2 := sb.Up(grpc.Address{Addr: endpoints[2]})
	addrFirst, putFun, err := sb.Get(context.Background(), unblockingOpts)
	if err != nil {
		t.Errorf("Get() with up endpoints should success, got %v", err)
	}
	if addrFirst.Addr != endpoints[1] {
		t.Errorf("Get() didn't return expected address, got %v", addrFirst)
	}
	if putFun == nil {
		t.Errorf("Get() returned unexpected nil put function")
	}
	addrSecond, _, _ := sb.Get(context.Background(), unblockingOpts)
	if addrFirst.Addr != addrSecond.Addr {
		t.Errorf("Get() didn't return the same address as previous call, got %v and %v", addrFirst, addrSecond)
	}

	down1(errors.New("error"))
	if addrs := <-sb.Notify(); len(addrs) != len(endpoints) {
		t.Errorf("closing the only connection should triggered balancer to send the all endpoints via Notify chan so that we can establish a connection")
	}
	down2(errors.New("error"))
	_, _, err = sb.Get(context.Background(), unblockingOpts)
	if err != ErrNoAddrAvilable {
		t.Errorf("Get() with no up endpoints should return ErrNoAddrAvailable, got: %v", err)
	}
}

func TestBalancerGetBlocking(t *testing.T) {
	sb := newSimpleBalancer(endpoints)
	if addrs := <-sb.Notify(); len(addrs) != len(endpoints) {
		t.Errorf("Initialize newSimpleBalancer should have triggered Notify() chan, but it didn't")
	}
	blockingOpts := grpc.BalancerGetOptions{BlockingWait: true}

	ctx, _ := context.WithTimeout(context.Background(), time.Millisecond*100)
	_, _, err := sb.Get(ctx, blockingOpts)
	if err != context.DeadlineExceeded {
		t.Errorf("Get() with no up endpoints should timeout, got %v", err)
	}

	downC := make(chan func(error), 1)

	go func() {
		// ensure sb.Up() will be called after sb.Get() to see if Up() releases blocking Get()
		time.Sleep(time.Millisecond * 100)
		downC <- sb.Up(grpc.Address{Addr: endpoints[1]})
		if addrs := <-sb.Notify(); len(addrs) != 1 {
			t.Errorf("first Up() should have triggered balancer to send the first connected address via Notify chan so that other connections can be closed")
		}
	}()
	addrFirst, putFun, err := sb.Get(context.Background(), blockingOpts)
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

	down2 := sb.Up(grpc.Address{Addr: endpoints[2]})
	addrSecond, _, _ := sb.Get(context.Background(), blockingOpts)
	if addrFirst.Addr != addrSecond.Addr {
		t.Errorf("Get() didn't return the same address as previous call, got %v and %v", addrFirst, addrSecond)
	}

	down1(errors.New("error"))
	if addrs := <-sb.Notify(); len(addrs) != len(endpoints) {
		t.Errorf("closing the only connection should triggered balancer to send the all endpoints via Notify chan so that we can establish a connection")
	}
	down2(errors.New("error"))
	ctx, _ = context.WithTimeout(context.Background(), time.Millisecond*100)
	_, _, err = sb.Get(ctx, blockingOpts)
	if err != context.DeadlineExceeded {
		t.Errorf("Get() with no up endpoints should timeout, got %v", err)
	}
}
