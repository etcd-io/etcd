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
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/version"
)

const (
	connPerPipeline = 4
	// pipelineBufSize is the size of pipeline buffer, which helps hold the
	// temporary network latency.
	// The size ensures that pipeline does not drop messages when the network
	// is out of work for less than 1 second in good path.
	pipelineBufSize = 64
)

type pipeline struct {
	from, to types.ID
	cid      types.ID

	tr     http.RoundTripper
	picker *urlPicker
	fs     *stats.FollowerStats
	r      Raft
	errorc chan error

	msgc chan raftpb.Message
	// wait for the handling routines
	wg sync.WaitGroup
	sync.Mutex
	// if the last send was successful, the pipeline is active.
	// Or it is inactive
	active  bool
	errored error
}

func newPipeline(tr http.RoundTripper, picker *urlPicker, from, to, cid types.ID, fs *stats.FollowerStats, r Raft, errorc chan error) *pipeline {
	p := &pipeline{
		from:   from,
		to:     to,
		cid:    cid,
		tr:     tr,
		picker: picker,
		fs:     fs,
		r:      r,
		errorc: errorc,
		msgc:   make(chan raftpb.Message, pipelineBufSize),
		active: true,
	}
	p.wg.Add(connPerPipeline)
	for i := 0; i < connPerPipeline; i++ {
		go p.handle()
	}
	return p
}

func (p *pipeline) stop() {
	close(p.msgc)
	p.wg.Wait()
}

func (p *pipeline) handle() {
	defer p.wg.Done()
	for m := range p.msgc {
		start := time.Now()
		err := p.post(pbutil.MustMarshal(&m))
		end := time.Now()

		p.Lock()
		if err != nil {
			reportSentFailure(pipelineMsg, m)

			if p.errored == nil || p.errored.Error() != err.Error() {
				log.Printf("pipeline: error posting to %s: %v", p.to, err)
				p.errored = err
			}
			if p.active {
				log.Printf("pipeline: the connection with %s became inactive", p.to)
				p.active = false
			}
			if m.Type == raftpb.MsgApp && p.fs != nil {
				p.fs.Fail()
			}
			p.r.ReportUnreachable(m.To)
			if isMsgSnap(m) {
				p.r.ReportSnapshot(m.To, raft.SnapshotFailure)
			}
		} else {
			if !p.active {
				log.Printf("pipeline: the connection with %s became active", p.to)
				p.active = true
				p.errored = nil
			}
			if m.Type == raftpb.MsgApp && p.fs != nil {
				p.fs.Succ(end.Sub(start))
			}
			if isMsgSnap(m) {
				p.r.ReportSnapshot(m.To, raft.SnapshotFinish)
			}
			reportSentDuration(pipelineMsg, m, time.Since(start))
		}
		p.Unlock()
	}
}

// post POSTs a data payload to a url. Returns nil if the POST succeeds,
// error on any failure.
func (p *pipeline) post(data []byte) error {
	u := p.picker.pick()
	uu := u
	uu.Path = RaftPrefix
	req, err := http.NewRequest("POST", uu.String(), bytes.NewBuffer(data))
	if err != nil {
		p.picker.unreachable(u)
		return err
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Server-From", p.from.String())
	req.Header.Set("X-Server-Version", version.Version)
	req.Header.Set("X-Min-Cluster-Version", version.MinClusterVersion)
	req.Header.Set("X-Etcd-Cluster-ID", p.cid.String())
	resp, err := p.tr.RoundTrip(req)
	if err != nil {
		p.picker.unreachable(u)
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		p.picker.unreachable(u)
		return err
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		switch strings.TrimSuffix(string(b), "\n") {
		case errIncompatibleVersion.Error():
			log.Printf("rafthttp: request sent was ignored by peer %s (server version incompatible)", p.to)
			return errIncompatibleVersion
		case errClusterIDMismatch.Error():
			log.Printf("rafthttp: request sent was ignored (cluster ID mismatch: remote[%s]=%s, local=%s)",
				p.to, resp.Header.Get("X-Etcd-Cluster-ID"), p.cid)
			return errClusterIDMismatch
		default:
			return fmt.Errorf("unhandled error %q when precondition failed", string(b))
		}
	case http.StatusForbidden:
		err := fmt.Errorf("the member has been permanently removed from the cluster")
		select {
		case p.errorc <- err:
		default:
		}
		return nil
	case http.StatusNoContent:
		return nil
	default:
		return fmt.Errorf("unexpected http status %s while posting to %q", http.StatusText(resp.StatusCode), req.URL.String())
	}
}
