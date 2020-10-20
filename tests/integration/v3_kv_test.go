package integration

import (
	"context"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
	"go.etcd.io/etcd/pkg/v3/testutil"
	"go.etcd.io/etcd/v3/embed"
	"go.etcd.io/etcd/v3/etcdserver/api/v3client"
	"io/ioutil"
	"os"
	"testing"
)

// TestKVWithEmptyValue ensures that a get/delete with an empty value, and with WithFromKey/WithPrefix function will return an empty error.
func TestKVWithEmptyValue(t *testing.T) {
	defer testutil.AfterTest(t)

	cfg := embed.NewConfig()

	// Use temporary data directory.
	dir, err := ioutil.TempDir("", "etcd-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	cfg.Dir = dir

	// Suppress server log to keep output clean.
	//cfg.Logger = "zap"
	//cfg.LogLevel = "error"

	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		panic(err)
	}
	defer etcd.Close()
	<-etcd.Server.ReadyNotify()

	client := v3client.New(etcd.Server)
	defer client.Close()

	_, err = client.Put(context.Background(), "my-namespace/foobar", "data")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Put(context.Background(), "my-namespace/foobar1", "data")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Put(context.Background(), "namespace/foobar1", "data")
	if err != nil {
		t.Fatal(err)
	}

	// Range over all keys.
	resp, err := client.Get(context.Background(), "", clientv3.WithFromKey())
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range resp.Kvs {
		t.Log(string(kv.Key), "=", string(kv.Value))
	}

	// Range over all keys in a namespace.
	client.KV = namespace.NewKV(client.KV, "my-namespace/")
	resp, err = client.Get(context.Background(), "", clientv3.WithFromKey())
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range resp.Kvs {
		t.Log(string(kv.Key), "=", string(kv.Value))
	}

	//Remove all keys without WithFromKey/WithPrefix func
	respDel, err := client.Delete(context.Background(), "")
	if err == nil {
		// fatal error duo to without WithFromKey/WithPrefix func called.
		t.Fatal(err)
	}

	respDel, err = client.Delete(context.Background(), "", clientv3.WithFromKey())
	if err != nil {
		// fatal error duo to with WithFromKey/WithPrefix func called.
		t.Fatal(err)
	}
	t.Logf("delete keys:%d", respDel.Deleted)
}
