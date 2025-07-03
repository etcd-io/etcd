// Copyright 2025 The etcd Authors
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

package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	clientv3 "go.etcd.io/etcd/client/v3"
	embed "go.etcd.io/etcd/server/v3/embed"
)

func TestWatchWithCache(t *testing.T) {
	client := startEtcd(t)
	cache, err := New(client, "/", WithHistoryWindowSize(32))
	if err != nil {
		t.Fatalf("New(...): %v", err)
	}
	t.Cleanup(cache.Close)
	if err := cache.WaitReady(t.Context()); err != nil {
		t.Fatalf("cache not ready: %v", err)
	}
	testWatch(t, client.KV, cache)
}

func TestWatchWithoutCache(t *testing.T) {
	client := startEtcd(t)
	testWatch(t, client.KV, client.Watcher)
}

func testWatch(t *testing.T, kv clientv3.KV, watcher Watcher) {
	ctx := t.Context()
	event1Put := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/a"),
			Value:          []byte("1"),
			CreateRevision: 2,
			ModRevision:    2,
			Version:        1,
		},
	}
	event2Put := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/b"),
			Value:          []byte("2"),
			CreateRevision: 3,
			ModRevision:    3,
			Version:        1,
		},
	}
	event3Delete := &clientv3.Event{
		Type: clientv3.EventTypeDelete,
		Kv: &mvccpb.KeyValue{
			Key:         []byte("/a"),
			ModRevision: 4,
		},
	}
	tcs := []struct {
		name       string
		key        string
		opts       []clientv3.OpOption
		wantEvents []*clientv3.Event
	}{
		{
			name:       "Watch all events",
			key:        "/",
			opts:       []clientv3.OpOption{clientv3.WithPrefix()},
			wantEvents: []*clientv3.Event{event1Put, event2Put, event3Delete},
		},
	}
	t.Log("Open test watchers")
	watches := make([]clientv3.WatchChan, len(tcs))
	for i, tc := range tcs {
		watches[i] = watcher.Watch(ctx, tc.key, tc.opts...)
	}
	t.Log("Setup data")
	if _, err := kv.Put(ctx, string(event1Put.Kv.Key), string(event1Put.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Put(ctx, string(event2Put.Kv.Key), string(event2Put.Kv.Value)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := kv.Delete(ctx, string(event3Delete.Kv.Key)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	t.Log("Validate")
	for i, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			events, _ := readEvents(watches[i])
			if diff := cmp.Diff(tc.wantEvents, events); diff != "" {
				t.Errorf("unexpected events (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLaggingWatcher(t *testing.T) {
	const prefix = "/test/"
	cli := startEtcd(t)

	tests := []struct {
		name       string
		window     int
		eventCount int
		wantEvents int
		wantClosed bool
	}{
		{
			name:       "all event fit",
			window:     10,
			eventCount: 9,
			wantEvents: 9,
			wantClosed: false,
		},
		{
			name:       "events fill window",
			window:     10,
			eventCount: 10,
			wantEvents: 10,
			wantClosed: false,
		},
		{
			name:       "event fill pipeline",
			window:     10,
			eventCount: 11,
			wantEvents: 11,
			wantClosed: false,
		},
		{
			name:       "pipeline overflow",
			window:     10,
			eventCount: 12,
			wantEvents: 1,
			wantClosed: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cache, err := New(
				cli, prefix,
				WithHistoryWindowSize(tt.window),
				WithPerWatcherBufferSize(0),
				WithResyncInterval(10*time.Millisecond),
			)
			if err != nil {
				t.Fatalf("New(...): %v", err)
			}
			defer cache.Close()

			if err := cache.WaitReady(t.Context()); err != nil {
				t.Fatalf("cache not ready: %v", err)
			}
			ch := cache.Watch(t.Context(), prefix, clientv3.WithPrefix())

			generateEvents(t, cli, prefix, tt.eventCount)
			gotEvents, ok := readEvents(ch)
			closed := !ok

			if tt.wantEvents != len(gotEvents) {
				t.Errorf("gotEvents=%v, wantEvents=%v", len(gotEvents), tt.wantEvents)
			}
			if closed != tt.wantClosed {
				t.Errorf("closed=%v, wantClosed=%v", closed, tt.wantClosed)
			}
		})
	}
}

func TestRejectsUnsupportedWatch(t *testing.T) {
	client := startEtcd(t)
	ctx := t.Context()

	cache, err := New(client, "")
	if err != nil {
		t.Fatalf("New(...): %v", err)
	}
	t.Cleanup(cache.Close)
	if err := cache.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		key  string
		opts []clientv3.OpOption
	}{
		{
			name: "non_zero_start_revision",
			key:  "",
			opts: []clientv3.OpOption{clientv3.WithPrefix(), clientv3.WithRev(123)},
		},
		{
			name: "subprefix_watch",
			key:  "foo/",
			opts: []clientv3.OpOption{clientv3.WithPrefix()},
		},
		{
			name: "single_key_watch",
			key:  "foo/bar",
			opts: nil, // exact-key watch (no WithPrefix)
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			watchCh := cache.Watch(ctx, tt.key, tt.opts...)
			resp, ok := <-watchCh

			if !ok || !resp.Canceled {
				t.Errorf("expected canceled response, got %#v (closed=%v)", resp, !ok)
			}
		})
	}
}

func startEtcd(t *testing.T) *clientv3.Client {
	t.Helper()

	cfg := embed.NewConfig()
	cfg.Dir = t.TempDir()
	cfg.LogLevel = "error"

	newClient, _ := types.NewURLs([]string{"http://127.0.0.1:0"})
	newPeer, _ := types.NewURLs([]string{"http://127.0.0.1:0"})
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = newClient, newClient
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = newPeer, newPeer

	cfg.InitialCluster = fmt.Sprintf("%s=%s", cfg.Name, cfg.AdvertisePeerUrls[0].String())

	srv, err := embed.StartEtcd(cfg)
	if err != nil {
		t.Fatalf("start etcd: %v", err)
	}
	t.Cleanup(func() { srv.Close() })

	select {
	case <-srv.Server.ReadyNotify():
	case <-time.After(10 * time.Second):
		t.Fatalf("etcd ready timeout")
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{srv.Clients[0].Addr().String()},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("client.Close() error: %v", err)
		}
	})
	return client
}

func generateEvents(t *testing.T, client *clientv3.Client, prefix string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		if _, err := client.Put(t.Context(), key, fmt.Sprintf("%d", i)); err != nil {
			t.Fatalf("Put(%q): %v", key, err)
		}
	}
}

type Watcher interface {
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
}

func readEvents(watch clientv3.WatchChan) (events []*clientv3.Event, ok bool) {
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case resp, ok := <-watch:
			if !ok {
				return events, false
			}
			events = append(events, resp.Events...)
		case <-deadline:
			return events, true
		}
	}
}
