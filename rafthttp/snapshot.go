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
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/pkg/ioutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
)

func newSnapshotKeeper(r Raft) *snapshotKeeper {
	return &snapshotKeeper{r: r}
}

func newSnapshotProcessor(tr http.RoundTripper, r Raft, snapSaver SnapshotSaver) *snapshotProcessor {
	return &snapshotProcessor{
		tr:        tr,
		r:         r,
		snapSaver: snapSaver,
	}
}

// snapshotKeeper keeps a snapshot for transporter. transporter
// needs to attach the member ID that the snapshot is sent to
// as soon as it knows.
// The kept snapshot can either be released by snapHandler, or
// be released automatically when it timeout. If snapshot is released
// automatically, snapshotKeeper also closes the snapshot and
// reports snapshot failure to raft.
// snapshotKeeper keeps at most one snapshot at a time.
type snapshotKeeper struct {
	r Raft

	// protect fields below
	mu sync.Mutex
	// index is the index of snapshot data for snapshot match
	// index is set to 0 if there is no kept snapshot
	index uint64
	// w is the WriterTo that can write the snapshot data
	w io.WriterTo
	// remote is the member ID that the kept snapshot will be sent to
	// remote is set to raft.None if the kept snapshot has not attached remote
	remote types.ID
	// when the closeTimer expires, it will call the function attached
	// via AfterFunc to close the snapshot
	// closeTimer is set to nil if the kept snapshot has no auto release
	closeTimer *time.Timer
}

// keep keeps the given snapshot at the given index.
// keep keeps at most one snapshot at a time, or it panics.
func (s *snapshotKeeper) keep(index uint64, w io.WriterTo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.index != 0 {
		plog.Panicf("unexpected keep snapshot when some snapshot is kept")
	}
	s.index = index
	s.w = w
	s.remote = types.ID(raft.None)
	s.closeTimer = nil
}

// keptIndex returns the index of the kept snapshot.
// If there is no kept snapshot, it returns 0.
func (s *snapshotKeeper) keptIndex() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index
}

// attachRemote attaches the given remote member ID to the kept snapshot.
func (s *snapshotKeeper) attachRemote(remote types.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remote = remote
}

// autoRelease releases the kept snapshot automatically after
// the given timeout passes.
// If the snapshot is released automatically, snapshotKeeper also closes
// the snapshot, and reports snapshot failure to raft.
// It MUST be called after remote has been attached.
func (s *snapshotKeeper) autoRelease(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeTimer = time.AfterFunc(timeout, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.index == 0 {
			return
		}
		plog.Infof("snapshot [index: %d, to: %s] failed to be sent out due to timeout", s.index, s.remote)
		s.index = 0
		s.w.WriteTo(&ioutil.ErrWriter{Err: errors.New("close snapshot due to timeout")})
		s.r.ReportSnapshot(uint64(s.remote), raft.SnapshotFailure)
	})
}

// release releases the kept snapshot with the given index, and stops
// potential auto release.
// If there is no available snapshot, release will return an error.
func (s *snapshotKeeper) release(index uint64) (w io.WriterTo, remote types.ID, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.index == 0 {
		return nil, 0, fmt.Errorf("no kept snapshot")
	}
	if s.index != index {
		return nil, 0, fmt.Errorf("no matched snapshot with index %d", index)
	}
	if s.remote == types.ID(raft.None) {
		return nil, 0, fmt.Errorf("no remote member ID attached to the kept snapshot")
	}
	s.index = 0
	if s.closeTimer != nil {
		s.closeTimer.Stop()
	}
	return s.w, s.remote, nil
}

// snapshotProcessor is used to process msgSnap.
// When the handler receives a msgSnap, it should pass it to snapshotProcessor.
// snapshotProcessor helps to get and save the snapshot data, then
// ask raft state machine to further process the message.
type snapshotProcessor struct {
	tr        http.RoundTripper
	r         Raft
	snapSaver SnapshotSaver
}

// process processes the given msgSnap.
// First, it requests the corresponding snapshot data from the given peerURLs.
// Then it saves the snapshot data into SnapshotStore. And finally, it sends the
// msgSnap to raft for further processing.
func (s *snapshotProcessor) process(m raftpb.Message, peerURLs types.URLs) {
	if m.Type != raftpb.MsgSnap {
		plog.Panicf("unexpected message type %v", m.Type)
	}

	// select one random peer URL to request snapshot
	//
	// Use one random URL is good enough because
	// 1. all peer URLs are supposed to function well
	// 2. snapshot sender keeps snapshot data for limited time
	// 3. snapshot sender will retry the process if this fails
	u := peerURLs[rand.Intn(len(peerURLs))]
	index := m.Snapshot.Metadata.Index
	snapr, err := s.getSnapFrom(u, index)
	if err != nil {
		plog.Warningf("failed to request snapshot [index: %d] (%v)", index, err)
		return
	}
	defer snapr.Close()

	if err := s.snapSaver.SaveFrom(snapr, index); err != nil {
		plog.Warningf("failed to read snapshot [index: %d] (%v)", index, err)
		return
	}
	if err := s.r.Process(context.TODO(), m); err != nil {
		plog.Warningf("failed to process raft message (%v)", err)
	}
}

// getSnapFrom requests the snapshot data at the given index from the given url.
// It returns a ReadCloser for the snapshot data when succeeds. Otherwise, an
// error will be returned.
// It is the caller's responsibility to close returned ReadCloser.
func (s *snapshotProcessor) getSnapFrom(u url.URL, index uint64) (io.ReadCloser, error) {
	u.Path = RaftSnapPrefix
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		plog.Panicf("unexpected new request error: %v", err)
	}
	req.Header.Set("X-Raft-Snapshot-Index", strconv.FormatUint(index, 10))
	resp, err := s.tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected http status %s", http.StatusText(resp.StatusCode))
	}
}
