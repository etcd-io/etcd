/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package rafthttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

func TestStopStreamServer(t *testing.T) {
	sender := &sender{}
	sh := NewStreamHandler(&resSenderFinder{sender}, 1, 1)
	srv := httptest.NewServer(sh)
	sc := newStreamClient(2, 1, 1, nil)
	if err := sc.start(http.DefaultTransport, srv.URL, 1); err != nil {
		t.Fatalf("start stream client error: %v", err)
	}
	sender.strmSrvMu.Lock()
	ss := sender.strmSrv
	sender.strmSrvMu.Unlock()

	sstimer := time.AfterFunc(time.Second, func() {
		t.Fatalf("stream server is not stopped after a long time")
	})
	ss.stop()
	sstimer.Stop()

	sctimer := time.AfterFunc(time.Second, func() {
		t.Fatalf("stream client is not stopped after a long time")
	})
	<-sc.done
	sctimer.Stop()
}

type resSenderFinder struct {
	s Sender
}

func (f *resSenderFinder) Sender(id types.ID) Sender {
	return f.s
}
