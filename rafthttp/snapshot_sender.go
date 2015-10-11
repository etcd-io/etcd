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
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/coreos/etcd/pkg/httputil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
)

type snapshotSender struct {
	from, to types.ID
	cid      types.ID

	tr     http.RoundTripper
	picker *urlPicker
	status *peerStatus
	snapst *snapshotStore
	r      Raft
	errorc chan error

	stopc chan struct{}
}

func newSnapshotSender(tr http.RoundTripper, picker *urlPicker, from, to, cid types.ID, status *peerStatus, snapst *snapshotStore, r Raft, errorc chan error) *snapshotSender {
	return &snapshotSender{
		from:   from,
		to:     to,
		cid:    cid,
		tr:     tr,
		picker: picker,
		status: status,
		snapst: snapst,
		r:      r,
		errorc: errorc,
		stopc:  make(chan struct{}),
	}
}

func (s *snapshotSender) stop() { close(s.stopc) }

func (s *snapshotSender) send(m raftpb.Message) {
	start := time.Now()

	body := createSnapBody(m, s.snapst)
	defer body.Close()

	u := s.picker.pick()
	req := createPostRequest(u, RaftSnapshotPrefix, body, "application/octet-stream", s.from, s.cid)

	err := s.post(req)
	if err != nil {
		// errMemberRemoved is a critical error since a removed member should
		// always be stopped. So we use reportCriticalError to report it to errorc.
		if err == errMemberRemoved {
			reportCriticalError(err, s.errorc)
		}
		s.picker.unreachable(u)
		reportSentFailure(sendSnap, m)
		s.status.deactivate(failureType{source: sendSnap, action: "post"}, err.Error())
		s.r.ReportUnreachable(m.To)
		// report SnapshotFailure to raft state machine. After raft state
		// machine knows about it, it would pause a while and retry sending
		// new snapshot message.
		s.r.ReportSnapshot(m.To, raft.SnapshotFailure)
		if s.status.isActive() {
			plog.Warningf("snapshot [index: %d, to: %s] failed to be sent out (%v)", m.Snapshot.Metadata.Index, types.ID(m.To), err)
		} else {
			plog.Debugf("snapshot [index: %d, to: %s] failed to be sent out (%v)", m.Snapshot.Metadata.Index, types.ID(m.To), err)
		}
		return
	}
	reportSentDuration(sendSnap, m, time.Since(start))
	s.status.activate()
	s.r.ReportSnapshot(m.To, raft.SnapshotFinish)
	plog.Infof("snapshot [index: %d, to: %s] sent out successfully", m.Snapshot.Metadata.Index, types.ID(m.To))
}

// post posts the given request.
// It returns nil when request is sent out and processed successfully.
func (s *snapshotSender) post(req *http.Request) (err error) {
	cancel := httputil.RequestCanceler(s.tr, req)

	type responseAndError struct {
		resp *http.Response
		body []byte
		err  error
	}
	result := make(chan responseAndError, 1)

	go func() {
		// TODO: cancel the request if it has waited for a long time(~5s) after
		// it has write out the full request body, which helps to avoid receiver
		// dies when sender is waiting for response
		// TODO: the snapshot could be large and eat up all resources when writing
		// it out. Send it block by block and rest some time between to give the
		// time for main loop to run.
		resp, err := s.tr.RoundTrip(req)
		if err != nil {
			result <- responseAndError{resp, nil, err}
			return
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		result <- responseAndError{resp, body, err}
	}()

	select {
	case <-s.stopc:
		cancel()
		return errStopped
	case r := <-result:
		if r.err != nil {
			return r.err
		}
		return checkPostResponse(r.resp, r.body, req, s.to)
	}
}

// readCloser implements io.ReadCloser interface.
type readCloser struct {
	io.Reader
	io.Closer
}

// createSnapBody creates the request body for the given raft snapshot message.
// Callers should close body when done reading from it.
func createSnapBody(m raftpb.Message, snapst *snapshotStore) io.ReadCloser {
	buf := new(bytes.Buffer)
	enc := &messageEncoder{w: buf}
	// encode raft message
	if err := enc.encode(m); err != nil {
		plog.Panicf("encode message error (%v)", err)
	}
	// get snapshot
	rc := snapst.get(m.Snapshot.Metadata.Index)

	return &readCloser{
		Reader: io.MultiReader(buf, rc),
		Closer: rc,
	}
}
