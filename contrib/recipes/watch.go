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

package recipe

import (
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/storage"
	"github.com/coreos/etcd/storage/storagepb"
)

type Watcher struct {
	wstream pb.Watch_WatchClient
	cancel  context.CancelFunc
	donec   chan struct{}
	id      storage.WatchID
	recvc   chan *storagepb.Event
	lastErr error
}

func NewWatcher(c *clientv3.Client, key string, rev int64) (*Watcher, error) {
	return newWatcher(c, key, rev, false)
}

func NewPrefixWatcher(c *clientv3.Client, prefix string, rev int64) (*Watcher, error) {
	return newWatcher(c, prefix, rev, true)
}

func newWatcher(c *clientv3.Client, key string, rev int64, isPrefix bool) (*Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	w, err := c.Watch.Watch(ctx)
	if err != nil {
		return nil, err
	}

	req := &pb.WatchCreateRequest{StartRevision: rev}
	if isPrefix {
		req.Prefix = []byte(key)
	} else {
		req.Key = []byte(key)
	}

	if err := w.Send(&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{CreateRequest: req}}); err != nil {
		return nil, err
	}

	wresp, err := w.Recv()
	if err != nil {
		return nil, err
	}
	if len(wresp.Events) != 0 || wresp.Created != true {
		return nil, ErrWaitMismatch
	}
	ret := &Watcher{
		wstream: w,
		cancel:  cancel,
		donec:   make(chan struct{}),
		id:      storage.WatchID(wresp.WatchId),
		recvc:   make(chan *storagepb.Event),
	}
	go ret.recvLoop()
	return ret, nil
}

func (w *Watcher) Close() error {
	defer w.cancel()
	if w.wstream == nil {
		return w.lastErr
	}
	req := &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CancelRequest{
		CancelRequest: &pb.WatchCancelRequest{
			WatchId: int64(w.id)}}}
	err := w.wstream.Send(req)
	if err != nil && w.lastErr == nil {
		return err
	}
	w.wstream.CloseSend()
	w.donec <- struct{}{}
	<-w.donec
	w.wstream = nil
	return w.lastErr
}

func (w *Watcher) Chan() <-chan *storagepb.Event { return w.recvc }

func (w *Watcher) recvLoop() {
	defer close(w.donec)
	for {
		wresp, err := w.wstream.Recv()
		if err != nil {
			w.lastErr = err
			break
		}
		for i := range wresp.Events {
			select {
			case <-w.donec:
				close(w.recvc)
				return
			case w.recvc <- wresp.Events[i]:
			}
		}
	}
	close(w.recvc)
	<-w.donec
}

func (w *Watcher) waitEvents(evs []storagepb.Event_EventType) (*storagepb.Event, error) {
	i := 0
	for {
		ev, ok := <-w.recvc
		if !ok {
			break
		}
		if ev.Type == evs[i] {
			i++
			if i == len(evs) {
				return ev, nil
			}
		}
	}
	return nil, w.Close()
}

// WaitEvents waits on a key until it observes the given events and returns the final one.
func WaitEvents(c *clientv3.Client, key string, rev int64, evs []storagepb.Event_EventType) (*storagepb.Event, error) {
	w, err := NewWatcher(c, key, rev)
	if err != nil {
		return nil, err
	}
	defer w.Close()
	return w.waitEvents(evs)
}

func WaitPrefixEvents(c *clientv3.Client, prefix string, rev int64, evs []storagepb.Event_EventType) (*storagepb.Event, error) {
	w, err := NewPrefixWatcher(c, prefix, rev)
	if err != nil {
		return nil, err
	}
	defer w.Close()
	return w.waitEvents(evs)
}
