package test

import (
	"context"
	"testing"
	"time"

	cachepkg "go.etcd.io/etcd/cache"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
)

func TestDemux(t *testing.T) {
	cfg := embed.NewConfig()
	cfg.Dir = t.TempDir()
	e, err := embed.StartEtcd(cfg)
	if err != nil {
		t.Fatalf("start etcd: %v", err)
	}
	defer e.Close()
	<-e.Server.ReadyNotify()

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{e.Clients[0].Addr().String()},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer cli.Close()

	// initial events
	for i := 0; i < 5; i++ {
		_, _ = cli.Put(context.Background(), "foo", "val")
	}

	c, err := cachepkg.New(cli,
		cachepkg.WithChannelSize(4),
		cachepkg.WithDropThreshold(5))
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	defer c.Close()

	// create watcher behind by 3 revs
	rev := c.CurrentRevision() - 3
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := c.Watch(ctx, "", rev)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	for i := 0; i < 3; i++ {
		<-s.C
	}

	// slow consumer test
	for i := 0; i < 10; i++ {
		_, _ = cli.Put(context.Background(), "foo", "slow")
	}

	time.Sleep(time.Second)

	// watcher should be dropped
	select {
	case _, ok := <-s.C:
		if ok {
			t.Fatalf("expected channel closed due to drop")
		}
	default:
		t.Fatalf("channel still open, drop failed")
	}
}
