// Copyright 2016 The etcd Authors
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

package grpcproxy

import (
	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type watchRange struct {
	key, end string
}

type watcher struct {
	id int64
	wr watchRange

	rev      int64
	filters  []mvcc.FilterFunc
	progress bool
	ch       chan<- *pb.WatchResponse
}

func (w *watcher) send(wr clientv3.WatchResponse) {
	if wr.IsProgressNotify() && !w.progress {
		return
	}

	events := make([]*mvccpb.Event, 0, len(wr.Events))

	for i := range wr.Events {
		ev := (*mvccpb.Event)(wr.Events[i])
		if ev.Kv.ModRevision <= w.rev {
			continue
		} else {
			w.rev = ev.Kv.ModRevision
		}

		filtered := false
		if len(w.filters) != 0 {
			for _, filter := range w.filters {
				if filter(*ev) {
					filtered = true
					break
				}
			}
		}

		if !filtered {
			events = append(events, ev)
		}
	}

	// all events are filtered out?
	if !wr.IsProgressNotify() && !wr.Created && len(events) == 0 {
		return
	}

	pbwr := &pb.WatchResponse{
		Header:  &wr.Header,
		Created: wr.Created,
		WatchId: w.id,
		Events:  events,
	}
	select {
	case w.ch <- pbwr:
	default:
		panic("handle this")
	}
}
