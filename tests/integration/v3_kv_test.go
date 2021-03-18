package integration

import (
	"context"
	"testing"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
)

// TestKVWithEmptyValue ensures that a get/delete with an empty value, and with WithFromKey/WithPrefix function will return an empty error.
func TestKVWithEmptyValue(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	client := clus.RandClient()

	_, err := client.Put(context.Background(), "my-namespace/foobar", "data")
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
	_, err = client.Delete(context.Background(), "")
	if err == nil {
		// fatal error duo to without WithFromKey/WithPrefix func called.
		t.Fatal(err)
	}

	respDel, err := client.Delete(context.Background(), "", clientv3.WithFromKey())
	if err != nil {
		// fatal error duo to with WithFromKey/WithPrefix func called.
		t.Fatal(err)
	}
	t.Logf("delete keys:%d", respDel.Deleted)
}
