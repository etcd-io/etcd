package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestWatch(t *testing.T) {
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
	watchTimeout := 2 * time.Second
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, tc.config)

			defer clus.Close()
			cc := clus.Client()
			testutils.ExecuteUntil(ctx, t, func() {
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
						wanted:   []testutils.KV{{Key: "bar", Val: "revision_2"}, {Key: "bar", Val: "revision_3"}},
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
					wCtx, wCancel := context.WithCancel(context.Background())
					wch := cc.Watch(wCtx, tt.watchKey, tt.opts)
					if wch == nil {
						t.Fatalf("failed to watch %s", tt.watchKey)
					}

					errs := make(chan error, 1)
					go func(i int, puts []testutils.KV) {
						defer close(errs)
						for j := range puts {
							if err := cc.Put(puts[j].Key, puts[j].Val, config.PutOptions{}); err != nil {
								errs <- fmt.Errorf("can't not put key %q, err: %s", puts[j].Key, err)
								break
							}
						}
						errs <- nil
						// sleep some time for watch
						time.Sleep(100 * time.Millisecond)
					}(i, tt.puts)

					err := <-errs
					if err != nil {
						t.Fatal(err)
					}

					var kvs []testutils.KV
				ForLoop:
					for {
						select {
						case watchResp, ok := <-wch:
							if ok {
								kvs = append(kvs, testutils.KeyValuesFromWatchResponse(watchResp)...)
								if len(kvs) == len(tt.wanted) {
									break ForLoop
								}
							}
						case <-time.After(watchTimeout):
							wCancel()
							t.Fatal("closed watcher channel should not block")
						}
					}

					wCancel()
					assert.Equal(t, tt.wanted, kvs)
				}
			})
		})
	}
}
