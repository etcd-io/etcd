package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/types"
	clientv3 "go.etcd.io/etcd/client/v3"
	embed "go.etcd.io/etcd/server/v3/embed"
)

type watchable interface {
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan
}

func TestCacheTwoWatchers(t *testing.T) {
	client := startEtcd(t)
	c, err := New(client, "", WithHistoryWindowSize(32))
	if err != nil {
		t.Fatalf("New(...) returned unexpected error: %v", err)
	}

	defer c.Close()

	ctx := t.Context()
	fooCh := c.Watch(ctx, "/foo/", clientv3.WithPrefix())
	barCh := c.Watch(ctx, "/bar/", clientv3.WithPrefix())

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := c.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Put(ctx, "/foo/a", "1"); err != nil {
		t.Fatalf("failed to put /foo/a: %v", err)
	}
	if _, err := client.Put(ctx, "/bar/b", "2"); err != nil {
		t.Fatalf("failed to put /bar/b: %v", err)
	}

	if event := waitEvent(t, fooCh); string(event.Kv.Key) != "/foo/a" {
		t.Fatalf("foo got %s", event.Kv.Key)
	}
	if event := waitEvent(t, barCh); string(event.Kv.Key) != "/bar/b" {
		t.Fatalf("bar got %s", event.Kv.Key)
	}
}

// testSeesEntireKeyspace asserts that a watcher registered on "\x00" receives an event for any key put afterwards.
func testSeesEntireKeyspace(t *testing.T, w watchable, writer *clientv3.Client) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	ch := w.Watch(ctx, "\x00", clientv3.WithFromKey())

	if _, err := writer.Put(ctx, "hello", "world"); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	select {
	case event := <-ch:
		if len(event.Events) == 0 || string(event.Events[0].Kv.Key) != "hello" {
			t.Fatalf("watcher delivered unexpected frame: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("watcher saw no events – wrong range?")
	}
}

func TestClientWatcherSeesEntireKeyspace(t *testing.T) {
	client := startEtcd(t)
	testSeesEntireKeyspace(t, client, client)
}

func TestCacheWatcherSeesEntireKeyspace(t *testing.T) {
	client := startEtcd(t)

	cache, err := New(client, "", WithHistoryWindowSize(32))
	if err != nil {
		t.Fatalf("New(...): %v", err)
	}
	defer cache.Close()

	if err := cache.WaitReady(context.Background()); err != nil {
		t.Fatalf("cache not ready: %v", err)
	}

	testSeesEntireKeyspace(t, cache, client)
}

func TestUnsupportedStartRev(t *testing.T) {
	client := startEtcd(t)
	ctx := t.Context()
	c, err := New(client, "")
	if err != nil {
		t.Fatalf("New(...) returned unexpected error: %v", err)
	}

	defer c.Close()
	if err := c.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}

	watchCh := c.Watch(ctx, "", clientv3.WithRev(123)) // any positive rev
	resp, ok := <-watchCh
	if !ok || !resp.Canceled {
		t.Fatalf("expected immediate Canceled response for unsupported startRev, got %#v", resp)
	}
}

// helpers
func startEtcd(t *testing.T) *clientv3.Client {
	t.Helper()

	cfg := embed.NewConfig()
	cfg.Dir = t.TempDir()
	cfg.LogLevel = "error"

	// pick free ports
	newClient, _ := types.NewURLs([]string{"http://127.0.0.1:0"})
	newPeer, _ := types.NewURLs([]string{"http://127.0.0.1:0"})
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = newClient, newClient
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = newPeer, newPeer

	// make the bootstrap string match the new peer URL
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

func waitEvent(t *testing.T, ch clientv3.WatchChan) clientv3.Event {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case watchResp, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed")
			}
			for _, event := range watchResp.Events {
				return *event // return the first still-unread event
			}
		case <-deadline:
			t.Fatalf("timeout")
		}
	}
}
