// Copyright 2015 CoreOS, Inc.
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

package storage

import (
	"sync"

	"github.com/coreos/etcd/storage/storagepb"
)

type watcher struct {
	key    []byte
	prefix bool
	cur    int64
	end    int64

	ch  chan storagepb.Event
	mu  sync.Mutex
	err error
}

func newWatcher(key []byte, prefix bool, start, end int64) *watcher {
	return &watcher{
		key:    key,
		prefix: prefix,
		cur:    start,
		end:    end,
		ch:     make(chan storagepb.Event, 10),
	}
}

func (w *watcher) Event() <-chan storagepb.Event { return w.ch }

func (w *watcher) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

func (w *watcher) stopWithError(err error) {
	if w.err != nil {
		return
	}
	close(w.ch)
	w.mu.Lock()
	w.err = err
	w.mu.Unlock()
}
