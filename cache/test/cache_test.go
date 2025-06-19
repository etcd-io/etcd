package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	cachepkg "go.etcd.io/etcd/cache"
	clientv3 "go.etcd.io/etcd/client/v3"
	embed "go.etcd.io/etcd/server/v3/embed"
)

// helpers

func startEtcd(t *testing.T) (*embed.Etcd, *clientv3.Client) {
	t.Helper()
	cfg := embed.NewConfig()
	cfg.Dir = t.TempDir()
	cfg.LogLevel = "error"
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
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{srv.Clients[0].Addr().String()},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { cli.Close() })
	return srv, cli
}

// waitEvent waits for a non‑empty WatchResponse up to 10s.
func waitEvent(t *testing.T, ch clientv3.WatchChan, why string) clientv3.Event {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case rsp, ok := <-ch:
			if !ok {
				t.Fatalf("%s: channel closed", why)
			}
			if len(rsp.Events) == 0 {
				continue
			}
			return *rsp.Events[0]
		case <-deadline:
			t.Fatalf("%s: timeout", why)
		}
	}
}

// two watchers see their own prefixes
func TestDemuxIsolation(t *testing.T) {
	_, cli := startEtcd(t)
	c, _ := cachepkg.NewWithPrefix(context.Background(), cli, "", cachepkg.WithHistoryWindowSize(32))
	defer c.Close()

	ctx := context.Background()
	fooCh := c.Watch(ctx, "/foo/", clientv3.WithPrefix())
	barCh := c.Watch(ctx, "/bar/", clientv3.WithPrefix())

	// give time for watcher registration
	time.Sleep(50 * time.Millisecond)

	cli.Put(ctx, "/foo/a", "1")
	cli.Put(ctx, "/bar/b", "2")

	if ev := waitEvent(t, fooCh, "foo event"); string(ev.Kv.Key) != "/foo/a" {
		t.Fatalf("foo got %s", ev.Kv.Key)
	}
	if ev := waitEvent(t, barCh, "bar event"); string(ev.Kv.Key) != "/bar/b" {
		t.Fatalf("bar got %s", ev.Kv.Key)
	}
}

// revision older than cache window returns compact frame then closes
func TestTooOldRevisionCompact(t *testing.T) {
	_, cli := startEtcd(t)
	c, _ := cachepkg.NewWithPrefix(context.Background(), cli, "", cachepkg.WithHistoryWindowSize(2))
	defer c.Close()
	ctx := context.Background()
	var revs []int64
	for i := 0; i < 5; i++ {
		resp, _ := cli.Put(ctx, fmt.Sprintf("k%d", i), "v")
		revs = append(revs, resp.Header.Revision)
		time.Sleep(10 * time.Millisecond)
	}
	oldestReq := revs[0]

	// wait until ring's oldest has definitely moved past oldestReq
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.OldestRev() > oldestReq {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	wch := c.Watch(ctx, "", clientv3.WithRev(oldestReq))
	select {
	case rsp, ok := <-wch:
		if !ok {
			t.Fatalf("compact watch closed w/o frame")
		}
		if rsp.CompactRevision == 0 {
			t.Fatalf("expected CompactRevision, got %#v", rsp)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for compact frame")
	}
	if _, more := <-wch; more {
		t.Fatalf("channel should close after compact frame")
	}
}

// watch remains usable after server-side Compact()
func TestWatchSurvivesServerCompaction(t *testing.T) {
	_, cli := startEtcd(t)
	c, _ := cachepkg.NewWithPrefix(context.Background(), cli, "")
	defer c.Close()
	ctx := context.Background()
	wch := c.Watch(ctx, "", clientv3.WithPrefix())

	// ensure watcher registered
	time.Sleep(50 * time.Millisecond)

	firstPut, _ := cli.Put(ctx, "alive", "1")
	waitEvent(t, wch, "initial put")

	cli.Compact(ctx, firstPut.Header.Revision)

	// give cache some time to observe compaction
	time.Sleep(100 * time.Millisecond)

	cli.Put(ctx, "after", "2")
	waitEvent(t, wch, "event after compaction")
}
