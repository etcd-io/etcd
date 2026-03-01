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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestStoreMetricsRestore(t *testing.T) {
	s := newStore(4, 8)
	s.Restore([]*mvccpb.KeyValue{
		makeKV("/a", "1", 5),
		makeKV("/b", "2", 5),
		makeKV("/c", "3", 5),
	}, 5)

	require.InEpsilon(t, 3, testutil.ToFloat64(storeKeysTotal), 0.01)
	require.InEpsilon(t, 5, testutil.ToFloat64(storeLatestRevision), 0.01)
}

func TestStoreMetricsApplyPutAndDelete(t *testing.T) {
	s := newStore(4, 8)
	s.Restore([]*mvccpb.KeyValue{
		makeKV("/a", "1", 5),
	}, 5)

	err := s.Apply(clientv3.WatchResponse{
		Events: []*clientv3.Event{
			makePutEvent("/b", "2", 6),
		},
	})
	require.NoError(t, err)
	require.InEpsilon(t, 2, testutil.ToFloat64(storeKeysTotal), 0.01)
	require.InEpsilon(t, 6, testutil.ToFloat64(storeLatestRevision), 0.01)

	err = s.Apply(clientv3.WatchResponse{
		Events: []*clientv3.Event{
			makeDelEvent("/a", 7),
		},
	})
	require.NoError(t, err)
	require.InEpsilon(t, 1, testutil.ToFloat64(storeKeysTotal), 0.01)
	require.InEpsilon(t, 7, testutil.ToFloat64(storeLatestRevision), 0.01)
}

func TestStoreMetricsProgressNotify(t *testing.T) {
	s := newStore(4, 8)
	s.Restore([]*mvccpb.KeyValue{makeKV("/a", "1", 5)}, 5)

	err := s.Apply(progressNotify(10))
	require.NoError(t, err)
	require.InEpsilon(t, 10, testutil.ToFloat64(storeLatestRevision), 0.01)
}

func TestDemuxMetricsRegisterUnregister(t *testing.T) {
	d := newDemux(8, 50*time.Millisecond)
	d.minRev = 1

	w1 := newWatcher(4, nil)
	w2 := newWatcher(4, nil)

	d.Register(w1, 0)
	require.InEpsilon(t, 1, testutil.ToFloat64(demuxActiveWatchers), 0.01)
	require.InDelta(t, 0, testutil.ToFloat64(demuxLaggingWatchers), 0)

	d.Register(w2, 0)
	require.InEpsilon(t, 2, testutil.ToFloat64(demuxActiveWatchers), 0.01)

	d.Unregister(w1)
	require.InEpsilon(t, 1, testutil.ToFloat64(demuxActiveWatchers), 0.01)

	d.Unregister(w2)
	require.InDelta(t, 0, testutil.ToFloat64(demuxActiveWatchers), 0)
}

func TestDemuxMetricsLaggingWatcher(t *testing.T) {
	d := newDemux(8, 1*time.Second)
	d.minRev = 1
	d.maxRev = 10

	w := newWatcher(4, nil)
	d.Register(w, 5)

	require.InDelta(t, 0, testutil.ToFloat64(demuxActiveWatchers), 0)
	require.InEpsilon(t, 1, testutil.ToFloat64(demuxLaggingWatchers), 0.01)

	d.Unregister(w)
	require.InDelta(t, 0, testutil.ToFloat64(demuxLaggingWatchers), 0)
}

func TestDemuxMetricsBroadcastUpdatesHistorySize(t *testing.T) {
	d := newDemux(8, 50*time.Millisecond)
	d.minRev = 1

	w := newWatcher(16, nil)
	d.Register(w, 0)

	err := d.Broadcast(clientv3.WatchResponse{
		Events: []*clientv3.Event{
			makePutEvent("/a", "1", 5),
			makePutEvent("/b", "2", 5),
		},
	})
	require.NoError(t, err)
	require.InEpsilon(t, 1, testutil.ToFloat64(demuxHistorySize), 0.01)

	err = d.Broadcast(clientv3.WatchResponse{
		Events: []*clientv3.Event{
			makePutEvent("/c", "3", 6),
		},
	})
	require.NoError(t, err)
	require.InEpsilon(t, 2, testutil.ToFloat64(demuxHistorySize), 0.01)

	d.Unregister(w)
}

func TestDemuxMetricsBroadcastOverflowPromotesToLagging(t *testing.T) {
	d := newDemux(8, 50*time.Millisecond)
	d.minRev = 1

	w := newWatcher(1, nil)
	d.Register(w, 0)

	err := d.Broadcast(clientv3.WatchResponse{
		Events: []*clientv3.Event{makePutEvent("/a", "1", 5)},
	})
	require.NoError(t, err)
	require.InEpsilon(t, 1, testutil.ToFloat64(demuxActiveWatchers), 0.01)

	err = d.Broadcast(clientv3.WatchResponse{
		Events: []*clientv3.Event{makePutEvent("/b", "2", 6)},
	})
	require.NoError(t, err)
	require.InDelta(t, 0, testutil.ToFloat64(demuxActiveWatchers), 0)
	require.InEpsilon(t, 1, testutil.ToFloat64(demuxLaggingWatchers), 0.01)

	d.Unregister(w)
}

func TestDemuxMetricsPurge(t *testing.T) {
	d := newDemux(8, 50*time.Millisecond)
	d.minRev = 1

	w := newWatcher(16, nil)
	d.Register(w, 0)

	_ = d.Broadcast(clientv3.WatchResponse{
		Events: []*clientv3.Event{makePutEvent("/a", "1", 5)},
	})

	require.InEpsilon(t, 1, testutil.ToFloat64(demuxActiveWatchers), 0.01)
	require.InEpsilon(t, 1, testutil.ToFloat64(demuxHistorySize), 0.01)

	d.Purge()

	require.InDelta(t, 0, testutil.ToFloat64(demuxActiveWatchers), 0)
	require.InDelta(t, 0, testutil.ToFloat64(demuxLaggingWatchers), 0)
	require.InDelta(t, 0, testutil.ToFloat64(demuxHistorySize), 0)
}

func TestHandleMetricsEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	HandleMetrics(mux, DefaultMetricsPath)

	req := httptest.NewRequest(http.MethodGet, DefaultMetricsPath, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"etcd_cache_store_keys_total",
		"etcd_cache_store_latest_revision",
		"etcd_cache_get_duration_seconds",
		"etcd_cache_get_total",
		"etcd_cache_watch_register_duration_seconds",
		"etcd_cache_watch_total",
		"etcd_cache_demux_active_watchers",
		"etcd_cache_demux_lagging_watchers",
		"etcd_cache_demux_history_size",
	} {
		require.Containsf(t, body, want, "expected %q in metrics output", want)
	}
}

func TestHandleMetricsCustomPath(t *testing.T) {
	mux := http.NewServeMux()
	HandleMetrics(mux, "/custom/metrics")

	req := httptest.NewRequest(http.MethodGet, "/custom/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "etcd_cache_store_keys_total")
}

func progressNotify(rev int64) clientv3.WatchResponse {
	return clientv3.WatchResponse{
		Header: etcdserverpb.ResponseHeader{Revision: rev},
	}
}
