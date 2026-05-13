// Copyright 2026 The etcd Authors
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

package cmd

import (
	"context"
	"strings"
	"testing"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestWaitForWatchChannelsReady(t *testing.T) {
	wchs := []clientv3.WatchChan{
		bufferedWatchChan(clientv3.WatchResponse{Created: true}),
		bufferedWatchChan(clientv3.WatchResponse{Created: true}),
	}

	if err := waitForWatchChannelsReady(wchs); err != nil {
		t.Fatalf("waitForWatchChannelsReady returned error: %v", err)
	}
}

func TestWaitForWatchChannelsReadyReturnsWatchError(t *testing.T) {
	wchs := []clientv3.WatchChan{
		bufferedWatchChan(clientv3.WatchResponse{
			Canceled:     true,
			CancelReason: "boom",
		}),
	}

	err := waitForWatchChannelsReady(wchs)
	if err == nil || !strings.Contains(err.Error(), "failed before watcher creation") {
		t.Fatalf("expected watcher creation failure, got %v", err)
	}
}

func TestCollectWatchLatencyEvents(t *testing.T) {
	previousBar := bar
	bar = nil
	defer func() {
		bar = previousBar
	}()

	eventTimes := make([]time.Time, 2)
	wch := make(chan clientv3.WatchResponse, 3)
	wch <- clientv3.WatchResponse{Header: &pb.ResponseHeader{Revision: 1}}
	wch <- clientv3.WatchResponse{Events: []*clientv3.Event{{}}}
	wch <- clientv3.WatchResponse{Events: []*clientv3.Event{{}}}
	close(wch)

	if err := collectWatchLatencyEvents(context.Background(), wch, 2, eventTimes); err != nil {
		t.Fatalf("collectWatchLatencyEvents returned error: %v", err)
	}
	if eventTimes[0].IsZero() || eventTimes[1].IsZero() {
		t.Fatalf("expected event timestamps to be recorded, got %v", eventTimes)
	}
}

func TestWatchLatencyRangeOptions(t *testing.T) {
	originalLimit := watchLRangeLimit
	originalConsistency := watchLRangeConsistency
	defer func() {
		watchLRangeLimit = originalLimit
		watchLRangeConsistency = originalConsistency
	}()

	watchLRangeLimit = 10
	watchLRangeConsistency = "l"
	_, mode, err := watchLatencyRangeOptions()
	if err != nil {
		t.Fatalf("watchLatencyRangeOptions(linearizable) returned error: %v", err)
	}
	if mode != "linearizable" {
		t.Fatalf("watchLatencyRangeOptions(linearizable) mode = %q, want %q", mode, "linearizable")
	}

	watchLRangeConsistency = "s"
	_, mode, err = watchLatencyRangeOptions()
	if err != nil {
		t.Fatalf("watchLatencyRangeOptions(serializable) returned error: %v", err)
	}
	if mode != "serializable" {
		t.Fatalf("watchLatencyRangeOptions(serializable) mode = %q, want %q", mode, "serializable")
	}

	watchLRangeConsistency = "x"
	if _, _, err := watchLatencyRangeOptions(); err == nil {
		t.Fatal("watchLatencyRangeOptions returned nil error for invalid consistency")
	}
}

func bufferedWatchChan(responses ...clientv3.WatchResponse) clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse, len(responses))
	for _, response := range responses {
		ch <- response
	}
	close(ch)
	return ch
}
