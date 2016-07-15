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
	"github.com/coreos/etcd/mvcc/mvccpb"
)

type watchRange struct {
	key, end string
}

type watcher struct {
	id int64
	wr watchRange
	// TODO: support filter
	progress bool
	ch       chan<- *pb.WatchResponse
}

func (w *watcher) send(wr clientv3.WatchResponse) {
	if wr.IsProgressNotify() && !w.progress {
		return
	}

	// todo: filter out the events that this watcher already seen.

	evs := wr.Events
	events := make([]*mvccpb.Event, len(evs))
	for i := range evs {
		events[i] = (*mvccpb.Event)(evs[i])
	}
	pbwr := &pb.WatchResponse{
		Header:  &wr.Header,
		WatchId: w.id,
		Events:  events,
	}
	select {
	case w.ch <- pbwr:
	default:
		panic("handle this")
	}
}
