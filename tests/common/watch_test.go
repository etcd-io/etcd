// Copyright 2022 The etcd Authors
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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	v3rpc "go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestWatch(t *testing.T) {
	testRunner.BeforeTest(t)
	watchTimeout := 1 * time.Second
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))

			defer clus.Close()
			cc := testutils.MustClient(clus.Client())
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
					require.NotNilf(t, wch, "failed to watch %s", tt.watchKey)

					for j := range tt.puts {
						_, err := cc.Put(ctx, tt.puts[j].Key, tt.puts[j].Val, config.PutOptions{})
						require.NoErrorf(t, err, "can't not put key %q, err: %s", tt.puts[j].Key, err)
					}

					kvs, err := testutils.KeyValuesFromWatchChan(wch, len(tt.wanted), watchTimeout)
					if err != nil {
						wCancel()
						require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
					}

					wCancel()
					assert.Equal(t, tt.wanted, kvs)
				}
			})
		})
	}
}

func TestStartWatcherFromCompactedRevision(t *testing.T) {
	t.Run("compaction on tombstone revision", func(t *testing.T) {
		testStartWatcherFromCompactedRevision(t, true)
	})
	t.Run("compaction on normal revision", func(t *testing.T) {
		testStartWatcherFromCompactedRevision(t, false)
	})
}

func testStartWatcherFromCompactedRevision(t *testing.T, performCompactOnTombstone bool) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))

			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			key := "foo"
			totalRev := 100

			type valueEvent struct {
				value string
				typ   mvccpb.Event_EventType
			}

			var (
				// requestedValues records all requested change
				requestedValues = make([]valueEvent, 0)
				// revisionChan sends each compacted revision via this channel
				compactionRevChan = make(chan int64)
				// compactionStep means that client performs a compaction on every 7 operations
				compactionStep = 7
			)

			// This goroutine will submit changes on $key $totalRev times. It will
			// perform compaction after every $compactedAfterChanges changes.
			// Except for first time, the watcher always receives the compacted
			// revision as start.
			go func() {
				defer close(compactionRevChan)

				lastRevision := int64(1)

				compactionRevChan <- lastRevision
				for vi := 1; vi <= totalRev; vi++ {
					var respHeader *etcdserverpb.ResponseHeader

					if vi%compactionStep == 0 && performCompactOnTombstone {
						t.Logf("DELETE key=%s", key)

						resp, derr := cc.Delete(ctx, key, config.DeleteOptions{})
						assert.NoError(t, derr)
						respHeader = resp.Header

						requestedValues = append(requestedValues, valueEvent{value: "", typ: mvccpb.DELETE})
					} else {
						value := fmt.Sprintf("%d", vi)

						t.Logf("PUT key=%s, val=%s", key, value)
						resp, perr := cc.Put(ctx, key, value, config.PutOptions{})
						assert.NoError(t, perr)
						respHeader = resp.Header

						requestedValues = append(requestedValues, valueEvent{value: value, typ: mvccpb.PUT})
					}

					lastRevision = respHeader.Revision

					if vi%compactionStep == 0 {
						compactionRevChan <- lastRevision

						t.Logf("COMPACT rev=%d", lastRevision)
						_, err := cc.Compact(ctx, lastRevision, config.CompactOption{Physical: true})
						assert.NoError(t, err)
					}
				}
			}()

			receivedEvents := make([]*clientv3.Event, 0)

			fromCompactedRev := false
			for fromRev := range compactionRevChan {
				wCtx, wCancel := context.WithTimeout(ctx, 30*time.Second)
				watchChan := cc.Watch(wCtx, key, config.WatchOptions{Revision: fromRev})

				prevEventCount := len(receivedEvents)

				// firstReceived represents this is first watch response.
				// Just in case that ETCD sends event one by one.
				firstReceived := true

				t.Logf("Start to watch key %s starting from revision %d", key, fromRev)
			watchLoop:
				for {
					currentEventCount := len(receivedEvents)
					if currentEventCount-prevEventCount == compactionStep || currentEventCount == totalRev {
						break
					}

					select {
					case watchResp := <-watchChan:
						t.Logf("Receive the number of events: %d", len(watchResp.Events))
						for i := range watchResp.Events {
							ev := watchResp.Events[i]

							// If the $fromRev is the compacted revision,
							// the first event should be the same as the last event receives in last watch response.
							if firstReceived && fromCompactedRev {
								firstReceived = false

								last := receivedEvents[prevEventCount-1]

								assert.Equalf(t, last.Type, ev.Type,
									"last received event type %s, but got event type %s", last.Type, ev.Type)
								assert.Equalf(t, string(last.Kv.Key), string(ev.Kv.Key),
									"last received event key %s, but got event key %s", string(last.Kv.Key), string(ev.Kv.Key))
								assert.Equalf(t, string(last.Kv.Value), string(ev.Kv.Value),
									"last received event value %s, but got event value %s", string(last.Kv.Value), string(ev.Kv.Value))
								continue
							}
							receivedEvents = append(receivedEvents, ev)
						}

						if len(watchResp.Events) == 0 {
							require.Equal(t, v3rpc.ErrCompacted, watchResp.Err())
							break watchLoop
						}

					case <-time.After(10 * time.Second):
						wCancel()
						t.Fatal("timed out getting watch response")
					}
				}

				wCancel()
				fromCompactedRev = true
			}

			t.Logf("Received total number of events: %d", len(receivedEvents))
			require.Len(t, requestedValues, totalRev)
			require.Lenf(t, receivedEvents, totalRev, "should receive %d events", totalRev)
			for idx, expected := range requestedValues {
				ev := receivedEvents[idx]

				require.Equalf(t, expected.typ, ev.Type, "#%d expected event %s", idx, expected.typ)

				updatedKey := string(ev.Kv.Key)

				require.Equal(t, key, updatedKey)
				if expected.typ == mvccpb.PUT {
					updatedValue := string(ev.Kv.Value)
					require.Equal(t, expected.value, updatedValue)
				}
			}
		})
	}
}
