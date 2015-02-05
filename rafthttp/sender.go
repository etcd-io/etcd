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

package rafthttp

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	connPerSender = 4
	// senderBufSize is the size of sender buffer, which helps hold the
	// temporary network latency.
	// The size ensures that sender does not drop messages when the network
	// is out of work for less than 1 second in good path.
	senderBufSize = 64
)

type sender struct {
	id  types.ID
	cid types.ID

	tr http.RoundTripper
	// the url this sender post to
	u      string
	fs     *stats.FollowerStats
	errorc chan error

	q chan *raftpb.Message
	// wait for the handling routines
	wg sync.WaitGroup
	sync.Mutex
	// if the last send was successful, the sender is active.
	// Or it is inactive
	active  bool
	errored error
}

func newSender(tr http.RoundTripper, u string, id, cid types.ID, fs *stats.FollowerStats, errorc chan error) *sender {
	s := &sender{
		id:     id,
		cid:    cid,
		tr:     tr,
		u:      u,
		fs:     fs,
		errorc: errorc,
		q:      make(chan *raftpb.Message, senderBufSize),
		active: true,
	}
	s.wg.Add(connPerSender)
	for i := 0; i < connPerSender; i++ {
		go s.handle()
	}
	return s
}

func (s *sender) update(u string) { s.u = u }

func (s *sender) send(m raftpb.Message) error {
	// TODO: don't block. we should be able to have 1000s
	// of messages out at a time.
	select {
	case s.q <- &m:
		return nil
	default:
		log.Printf("sender: dropping %s because maximal number %d of sender buffer entries to %s has been reached",
			m.Type, senderBufSize, s.u)
		return fmt.Errorf("reach maximal serving")
	}
}

func (s *sender) stop() {
	close(s.q)
	s.wg.Wait()
}

func (s *sender) handle() {
	defer s.wg.Done()
	for m := range s.q {
		start := time.Now()
		err := s.post(pbutil.MustMarshal(m))
		end := time.Now()

		s.Lock()
		if err != nil {
			if s.errored == nil || s.errored.Error() != err.Error() {
				log.Printf("sender: error posting to %s: %v", s.id, err)
				s.errored = err
			}
			if s.active {
				log.Printf("sender: the connection with %s becomes inactive", s.id)
				s.active = false
			}
			if m.Type == raftpb.MsgApp {
				s.fs.Fail()
			}
		} else {
			if !s.active {
				log.Printf("sender: the connection with %s becomes active", s.id)
				s.active = true
				s.errored = nil
			}
			if m.Type == raftpb.MsgApp {
				s.fs.Succ(end.Sub(start))
			}
		}
		s.Unlock()
	}
}

// post POSTs a data payload to a url. Returns nil if the POST succeeds,
// error on any failure.
func (s *sender) post(data []byte) error {
	s.Lock()
	req, err := http.NewRequest("POST", s.u, bytes.NewBuffer(data))
	s.Unlock()
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Etcd-Cluster-ID", s.cid.String())
	resp, err := s.tr.RoundTrip(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		err := fmt.Errorf("conflicting cluster ID with the target cluster (%s != %s)", resp.Header.Get("X-Etcd-Cluster-ID"), s.cid)
		select {
		case s.errorc <- err:
		default:
		}
		return nil
	case http.StatusForbidden:
		err := fmt.Errorf("the member has been permanently removed from the cluster")
		select {
		case s.errorc <- err:
		default:
		}
		return nil
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}
