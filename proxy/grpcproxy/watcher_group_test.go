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
	"testing"

	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

func TestWatchgroupBroadcast(t *testing.T) {
	wch := make(chan clientv3.WatchResponse, 0)
	wg := newWatchergroup(wch, nil)
	go wg.run()

	chs := make([]chan *pb.WatchResponse, 10)
	for i := range chs {
		chs[i] = make(chan *pb.WatchResponse, 1)
		w := watcher{
			id: int64(i),
			ch: chs[i],

			progress: true,
		}
		rid := receiverID{streamID: 1, watcherID: w.id}
		wg.add(rid, w)
	}

	// send a progress response
	wch <- clientv3.WatchResponse{Header: pb.ResponseHeader{Revision: 1}}

	for _, ch := range chs {
		<-ch
	}
}
