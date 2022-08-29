package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestWatch(t *testing.T) {
	testRunner.BeforeTest(t)
	watchTimeout := 1 * time.Second
	for _, tc := range clusterTestCases {
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

				for _, tt := range tests {
					wCtx, wCancel := context.WithCancel(ctx)
					wch := cc.Watch(wCtx, tt.watchKey, tt.opts)
					if wch == nil {
						t.Fatalf("failed to watch %s", tt.watchKey)
					}

					for j := range tt.puts {
						if err := cc.Put(ctx, tt.puts[j].Key, tt.puts[j].Val, config.PutOptions{}); err != nil {
							t.Fatalf("can't not put key %q, err: %s", tt.puts[j].Key, err)
						}
					}

					kvs, err := testutils.KeyValuesFromWatchChan(wch, len(tt.wanted), watchTimeout)
					if err != nil {
						wCancel()
						t.Fatalf("failed to get key-values from watch channel %s", err)
					}

					wCancel()
					assert.Equal(t, tt.wanted, kvs)
				}
			})
		})
	}
}
