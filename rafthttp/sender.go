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
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/types"
)

const (
	connPerSender = 4
	senderBufSize = connPerSender * 4
)

type Sender interface {
	Update(u string)
	// Send sends the data to the remote node. It is always non-blocking.
	// It may be fail to send data if it returns nil error.
	Send(data []byte) error
	// Stop performs any necessary finalization and terminates the Sender
	// elegantly.
	Stop()
}

func NewSender(tr http.RoundTripper, u string, cid types.ID, fs *stats.FollowerStats, shouldstop chan struct{}) *sender {
	s := &sender{
		tr:         tr,
		u:          u,
		cid:        cid,
		fs:         fs,
		q:          make(chan []byte, senderBufSize),
		shouldstop: shouldstop,
	}
	s.wg.Add(connPerSender)
	for i := 0; i < connPerSender; i++ {
		go s.handle()
	}
	return s
}

type sender struct {
	tr         http.RoundTripper
	u          string
	cid        types.ID
	fs         *stats.FollowerStats
	q          chan []byte
	mu         sync.RWMutex
	wg         sync.WaitGroup
	shouldstop chan struct{}
}

func (s *sender) Update(u string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.u = u
}

// TODO (xiangli): reasonable retry logic
func (s *sender) Send(data []byte) error {
	select {
	case s.q <- data:
		return nil
	default:
		log.Printf("sender: reach the maximal serving to %s", s.u)
		return fmt.Errorf("reach maximal serving")
	}
}

func (s *sender) Stop() {
	close(s.q)
	s.wg.Wait()
}

func (s *sender) handle() {
	defer s.wg.Done()
	for d := range s.q {
		start := time.Now()
		err := s.post(d)
		end := time.Now()
		if err != nil {
			s.fs.Fail()
			log.Printf("sender: %v", err)
			continue
		}
		s.fs.Succ(end.Sub(start))
	}
}

// post POSTs a data payload to a url. Returns nil if the POST succeeds,
// error on any failure.
func (s *sender) post(data []byte) error {
	s.mu.RLock()
	req, err := http.NewRequest("POST", s.u, bytes.NewBuffer(data))
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("new request to %s error: %v", s.u, err)
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Etcd-Cluster-ID", s.cid.String())
	resp, err := s.tr.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("error posting to %q: %v", req.URL.String(), err)
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		select {
		case s.shouldstop <- struct{}{}:
		default:
		}
		log.Printf("etcdserver: conflicting cluster ID with the target cluster (%s != %s)", resp.Header.Get("X-Etcd-Cluster-ID"), s.cid)
		return nil
	case http.StatusForbidden:
		select {
		case s.shouldstop <- struct{}{}:
		default:
		}
		log.Println("etcdserver: this member has been permanently removed from the cluster")
		log.Println("etcdserver: the data-dir used by this member must be removed so that this host can be re-added with a new member ID")
		return nil
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unhandled status %s", http.StatusText(resp.StatusCode))
	}
}
