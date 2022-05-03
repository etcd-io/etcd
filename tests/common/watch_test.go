package common

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestTxn(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 1, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	watchTimeout := 3 * time.Second
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()
			testutils.ExecuteWithTimeout(t, 20*time.Second, func() {
				tests := []struct {
					puts     []testutils.KV
					watchKey string
					opts     config.WatchOptions
					wanted   []testutils.KV
				}{
					{ // watch by revision
						puts:     []testutils.KV{{Key: "bar", Val: "revision_1"}, {Key: "bar", Val: "revision_2"}, {Key: "bar", Val: "revision_3"}},
						watchKey: "bar",
						opts:     config.WatchOptions{Revision: 3},
						wanted:   []testutils.KV{{Key: "bar2", Val: "revision_2"}, {Key: "bar", Val: "revision_3"}},
					},
					{ // watch 1 key
						puts:     []testutils.KV{{Key: "sample", Val: "value"}},
						watchKey: "sample",
						opts:     config.WatchOptions{Revision: 1},
						wanted:   []testutils.KV{{Key: "sample", Val: "value"}},
					},
					{ // watch 3 keys by prefix
						puts:     []testutils.KV{{Key: "foo1", Val: "val1"}, {Key: "foo2", Val: "val2"}, {Key: "foo3", Val: "val3"}},
						watchKey: "foo",
						opts:     config.WatchOptions{Revision: 1, Prefix: true},
						wanted:   []testutils.KV{{Key: "foo1", Val: "val1"}, {Key: "foo2", Val: "val2"}, {Key: "foo3", Val: "val3"}},
					},
					{ // watch 3 keys by range
						puts:     []testutils.KV{{Key: "key1", Val: "val1"}, {Key: "key3", Val: "val3"}, {Key: "key2", Val: "val2"}},
						watchKey: "key",
						opts:     config.WatchOptions{Revision: 1, RangeEnd: "key3"},
						wanted:   []testutils.KV{{Key: "key1", Val: "val1"}, {Key: "key2", Val: "val2"}},
					},
				}

				for i, tt := range tests {
					var wg sync.WaitGroup
					wg.Add(1)
					errs := make(chan error, 1)
					ctx, cancel := context.WithCancel(context.Background())
					go func(i int, puts []testutils.KV) {
						defer wg.Done()
						defer close(errs)
						defer cancel()
						for j := range puts {
							if err := cc.Put(puts[j].Key, puts[j].Val, config.PutOptions{}); err != nil {
								errs <- fmt.Errorf("can't not put key %q, err: %s", puts[j].Key, err)
								break
							}
							time.Sleep(time.Second)
						}
						errs <- nil
					}(i, tt.puts)

					err := <-errs
					if err != nil {
						t.Fatal(err)
					}
					wg.Wait()

					wch := cc.Watch(ctx, tt.watchKey, tt.opts)
					select {
					case wresp, ok := <-wch:
						if ok {
							kvs := testutils.KeyValuesFromWatchResponse(wresp)
							assert.Equal(t, tt.wanted, kvs)
						}
					case <-time.After(watchTimeout):
						t.Fatalf("closed watcher channel should not block")
					}
				}
			})
		})
	}
}
