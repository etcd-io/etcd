// Copyright 2016 CoreOS, Inc.
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

package integration

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
	storagepb "github.com/coreos/etcd/storage/storagepb"
)

type watcherTest func(*testing.T, *watchctx)

type watchctx struct {
	clus    *integration.ClusterV3
	w       clientv3.Watcher
	wclient *clientv3.Client
	kv      clientv3.KV
	ch      <-chan clientv3.WatchResponse
}

func runWatchTest(t *testing.T, f watcherTest) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	wclient := clus.RandClient()
	w := clientv3.NewWatcher(wclient)
	defer w.Close()
	// select a different client from wclient so puts succeed if
	// a test knocks out the watcher client
	kvclient := clus.RandClient()
	for kvclient == wclient {
		kvclient = clus.RandClient()
	}
	kv := clientv3.NewKV(kvclient)

	wctx := &watchctx{clus, w, wclient, kv, nil}
	f(t, wctx)
}

// TestWatchMultiWatcher modifies multiple keys and observes the changes.
func TestWatchMultiWatcher(t *testing.T) {
	runWatchTest(t, testWatchMultiWatcher)
}

func testWatchMultiWatcher(t *testing.T, wctx *watchctx) {
	numKeyUpdates := 4
	keys := []string{"foo", "bar", "baz"}

	donec := make(chan struct{})
	readyc := make(chan struct{})
	for _, k := range keys {
		// key watcher
		go func(key string) {
			ch := wctx.w.Watch(context.TODO(), key, 0)
			if ch == nil {
				t.Fatalf("expected watcher channel, got nil")
			}
			readyc <- struct{}{}
			for i := 0; i < numKeyUpdates; i++ {
				resp, ok := <-ch
				if !ok {
					t.Fatalf("watcher unexpectedly closed")
				}
				v := fmt.Sprintf("%s-%d", key, i)
				gotv := string(resp.Events[0].Kv.Value)
				if gotv != v {
					t.Errorf("#%d: got %s, wanted %s", i, gotv, v)
				}
			}
			donec <- struct{}{}
		}(k)
	}
	// prefix watcher on "b" (bar and baz)
	go func() {
		prefixc := wctx.w.WatchPrefix(context.TODO(), "b", 0)
		if prefixc == nil {
			t.Fatalf("expected watcher channel, got nil")
		}
		readyc <- struct{}{}
		evs := []*storagepb.Event{}
		for i := 0; i < numKeyUpdates*2; i++ {
			resp, ok := <-prefixc
			if !ok {
				t.Fatalf("watcher unexpectedly closed")
			}
			evs = append(evs, resp.Events...)
		}

		// check response
		expected := []string{}
		bkeys := []string{"bar", "baz"}
		for _, k := range bkeys {
			for i := 0; i < numKeyUpdates; i++ {
				expected = append(expected, fmt.Sprintf("%s-%d", k, i))
			}
		}
		got := []string{}
		for _, ev := range evs {
			got = append(got, string(ev.Kv.Value))
		}
		sort.Strings(got)
		if reflect.DeepEqual(expected, got) == false {
			t.Errorf("got %v, expected %v", got, expected)
		}

		// ensure no extra data
		select {
		case resp, ok := <-prefixc:
			if !ok {
				t.Fatalf("watcher unexpectedly closed")
			}
			t.Fatalf("unexpected event %+v", resp)
		case <-time.After(time.Second):
		}
		donec <- struct{}{}
	}()

	// wait for watcher bring up
	for i := 0; i < len(keys)+1; i++ {
		<-readyc
	}
	// generate events
	for i := 0; i < numKeyUpdates; i++ {
		for _, k := range keys {
			v := fmt.Sprintf("%s-%d", k, i)
			if _, err := wctx.kv.Put(k, v, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	// wait for watcher shutdown
	for i := 0; i < len(keys)+1; i++ {
		<-donec
	}
}

// TestWatchReconnInit tests watcher resumes correctly if connection lost
// before any data was sent.
func TestWatchReconnInit(t *testing.T) {
	runWatchTest(t, testWatchReconnInit)
}

func testWatchReconnInit(t *testing.T, wctx *watchctx) {
	if wctx.ch = wctx.w.Watch(context.TODO(), "a", 0); wctx.ch == nil {
		t.Fatalf("expected non-nil channel")
	}
	// take down watcher connection
	wctx.wclient.ActiveConnection().Close()
	// watcher should recover
	putAndWatch(t, wctx, "a", "a")
}

// TestWatchReconnRunning tests watcher resumes correctly if connection lost
// after data was sent.
func TestWatchReconnRunning(t *testing.T) {
	runWatchTest(t, testWatchReconnRunning)
}

func testWatchReconnRunning(t *testing.T, wctx *watchctx) {
	if wctx.ch = wctx.w.Watch(context.TODO(), "a", 0); wctx.ch == nil {
		t.Fatalf("expected non-nil channel")
	}
	putAndWatch(t, wctx, "a", "a")
	// take down watcher connection
	wctx.wclient.ActiveConnection().Close()
	// watcher should recover
	putAndWatch(t, wctx, "a", "b")
}

// TestWatchCancelInit tests watcher closes correctly after no events.
func TestWatchCancelInit(t *testing.T) {
	runWatchTest(t, testWatchCancelInit)
}

func testWatchCancelInit(t *testing.T, wctx *watchctx) {
	ctx, cancel := context.WithCancel(context.Background())
	if wctx.ch = wctx.w.Watch(ctx, "a", 0); wctx.ch == nil {
		t.Fatalf("expected non-nil watcher channel")
	}
	cancel()
	select {
	case <-time.After(time.Second):
		t.Fatalf("took too long to cancel")
	case _, ok := <-wctx.ch:
		if ok {
			t.Fatalf("expected watcher channel to close")
		}
	}
}

// TestWatchCancelRunning tests watcher closes correctly after events.
func TestWatchCancelRunning(t *testing.T) {
	runWatchTest(t, testWatchCancelRunning)
}

func testWatchCancelRunning(t *testing.T, wctx *watchctx) {
	ctx, cancel := context.WithCancel(context.Background())
	if wctx.ch = wctx.w.Watch(ctx, "a", 0); wctx.ch == nil {
		t.Fatalf("expected non-nil watcher channel")
	}
	if _, err := wctx.kv.Put("a", "a", 0); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-time.After(time.Second):
		t.Fatalf("took too long to cancel")
	case v, ok := <-wctx.ch:
		if !ok {
			// closed before getting put; OK
			break
		}
		// got the PUT; should close next
		select {
		case <-time.After(time.Second):
			t.Fatalf("took too long to close")
		case v, ok = <-wctx.ch:
			if ok {
				t.Fatalf("expected watcher channel to close, got %v", v)
			}
		}
	}
}

func putAndWatch(t *testing.T, wctx *watchctx, key, val string) {
	if _, err := wctx.kv.Put(key, val, 0); err != nil {
		t.Fatal(err)
	}
	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("watch timed out")
	case v, ok := <-wctx.ch:
		if !ok {
			t.Fatalf("unexpected watch close")
		}
		if string(v.Events[0].Kv.Value) != val {
			t.Fatalf("bad value got %v, wanted %v", v.Events[0].Kv.Value, val)
		}
	}
}
