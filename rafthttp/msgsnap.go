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
	"encoding/binary"
	"io"
	"time"

	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	// 1MB usually could be finished in 100ms, which is the default value of
	// heartbeat interval. rafthttp rests some time after transferring one
	// block, so it leaves time for leader to send heartbeat to follower.
	transferBlockSize = 1024 * 1024
	// sleepRate specifies the rate between sleep time and send time in encoder.
	sleepRate = 1
)

type msgSnapEncoder struct {
	w       io.Writer
	snaphub SnapshotHub
}

// TODO: cancel old term message when higher term happens, which helps efficiency.
func (enc *msgSnapEncoder) encode(m raftpb.Message) error {
	if isLinkHeartbeatMessage(m) {
		return binary.Write(enc.w, binary.BigEndian, uint64(0))
	}

	if err := binary.Write(enc.w, binary.BigEndian, uint64(m.Size())); err != nil {
		return err
	}
	if _, err := enc.w.Write(pbutil.MustMarshal(&m)); err != nil {
		return err
	}
	rc, size := enc.snaphub.SnapshotData(m.Snapshot)
	defer rc.Close()
	if err := binary.Write(enc.w, binary.BigEndian, uint64(size)); err != nil {
		return err
	}
	for {
		lr := &io.LimitedReader{R: rc, N: transferBlockSize}
		start := time.Now()
		_, err := io.Copy(enc.w, lr)
		if err != nil {
			return err
		}
		if lr.N == transferBlockSize {
			break
		}
		time.Sleep(time.Since(start) * time.Duration(sleepRate))
	}
	return nil
}

type msgSnapDecoder struct {
	r       io.Reader
	snaphub SnapshotHub
}

func (dec *msgSnapDecoder) decode() (raftpb.Message, error) {
	var m raftpb.Message
	var l uint64
	if err := binary.Read(dec.r, binary.BigEndian, &l); err != nil {
		return m, err
	}
	if l == 0 {
		return linkHeartbeatMessage, nil
	}

	buf := make([]byte, int(l))
	if _, err := io.ReadFull(dec.r, buf); err != nil {
		return m, err
	}
	if err := m.Unmarshal(buf); err != nil {
		return m, err
	}
	var size int64
	if err := binary.Read(dec.r, binary.BigEndian, &size); err != nil {
		return m, err
	}
	lr := &io.LimitedReader{R: dec.r, N: size}
	err := dec.snaphub.SaveSnapshotData(m.Snapshot, lr)
	return m, err
}
