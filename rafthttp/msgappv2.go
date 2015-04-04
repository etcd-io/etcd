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
	"fmt"
	"io"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

const (
	msgTypeLinkHeartbeat uint8 = 0
	msgTypeAppEntries    uint8 = 1
	msgTypeApp           uint8 = 2
)

// msgappv2 stream sends three types of message: linkHeartbeatMessage,
// AppEntries and MsgApp. AppEntries is the MsgApp that is sent in
// replicate state in raft, whose index and term are fully predicatable.
//
// Data format of linkHeartbeatMessage:
// | offset | bytes | description |
// +--------+-------+-------------+
// | 0      | 1     | \x00        |
//
// Data format of AppEntries:
// | offset | bytes | description |
// +--------+-------+-------------+
// | 0      | 1     | \x01        |
// | 1      | 8     | length of entries |
// | 9      | 8     | length of first entry |
// | 17     | n1    | first entry |
// ...
// | x      | 8     | length of k-th entry data |
// | x+8    | nk    | k-th entry data |
// | x+8+nk | 8     | commit index |
//
// Data format of MsgApp:
// | offset | bytes | description |
// +--------+-------+-------------+
// | 0      | 1     | \x01        |
// | 1      | 8     | length of encoded message |
// | 9      | n     | encoded message |
type msgAppV2Encoder struct {
	w  io.Writer
	fs *stats.FollowerStats

	term  uint64
	index uint64
}

func (enc *msgAppV2Encoder) encode(m raftpb.Message) error {
	start := time.Now()
	switch {
	case isLinkHeartbeatMessage(m):
		return binary.Write(enc.w, binary.BigEndian, msgTypeLinkHeartbeat)
	case enc.index == m.Index && enc.term == m.LogTerm && m.LogTerm == m.Term:
		if err := binary.Write(enc.w, binary.BigEndian, msgTypeAppEntries); err != nil {
			return err
		}
		// write length of entries
		l := len(m.Entries)
		if err := binary.Write(enc.w, binary.BigEndian, uint64(l)); err != nil {
			return err
		}
		for i := 0; i < l; i++ {
			size := m.Entries[i].Size()
			if err := binary.Write(enc.w, binary.BigEndian, uint64(size)); err != nil {
				return err
			}
			if _, err := enc.w.Write(pbutil.MustMarshal(&m.Entries[i])); err != nil {
				return err
			}
			enc.index++
		}
		// write commit index
		if err := binary.Write(enc.w, binary.BigEndian, m.Commit); err != nil {
			return err
		}
	default:
		if err := binary.Write(enc.w, binary.BigEndian, msgTypeApp); err != nil {
			return err
		}
		// write size of message
		if err := binary.Write(enc.w, binary.BigEndian, uint64(m.Size())); err != nil {
			return err
		}
		// write message
		if _, err := enc.w.Write(pbutil.MustMarshal(&m)); err != nil {
			return err
		}

		enc.term = m.Term
		enc.index = m.Index
		if l := len(m.Entries); l > 0 {
			enc.index = m.Entries[l-1].Index
		}
	}
	enc.fs.Succ(time.Since(start))
	return nil
}

type msgAppV2Decoder struct {
	r             io.Reader
	local, remote types.ID

	term  uint64
	index uint64
}

func (dec *msgAppV2Decoder) decode() (raftpb.Message, error) {
	var (
		m   raftpb.Message
		typ uint8
	)
	if err := binary.Read(dec.r, binary.BigEndian, &typ); err != nil {
		return m, err
	}
	switch typ {
	case msgTypeLinkHeartbeat:
		return linkHeartbeatMessage, nil
	case msgTypeAppEntries:
		m = raftpb.Message{
			Type:    raftpb.MsgApp,
			From:    uint64(dec.remote),
			To:      uint64(dec.local),
			Term:    dec.term,
			LogTerm: dec.term,
			Index:   dec.index,
		}

		// decode entries
		var l uint64
		if err := binary.Read(dec.r, binary.BigEndian, &l); err != nil {
			return m, err
		}
		m.Entries = make([]raftpb.Entry, int(l))
		for i := 0; i < int(l); i++ {
			var size uint64
			if err := binary.Read(dec.r, binary.BigEndian, &size); err != nil {
				return m, err
			}
			buf := make([]byte, int(size))
			if _, err := io.ReadFull(dec.r, buf); err != nil {
				return m, err
			}
			dec.index++
			pbutil.MustUnmarshal(&m.Entries[i], buf)
		}
		// decode commit index
		if err := binary.Read(dec.r, binary.BigEndian, &m.Commit); err != nil {
			return m, err
		}
	case msgTypeApp:
		var size uint64
		if err := binary.Read(dec.r, binary.BigEndian, &size); err != nil {
			return m, err
		}
		buf := make([]byte, int(size))
		if _, err := io.ReadFull(dec.r, buf); err != nil {
			return m, err
		}
		pbutil.MustUnmarshal(&m, buf)

		dec.term = m.Term
		dec.index = m.Index
		if l := len(m.Entries); l > 0 {
			dec.index = m.Entries[l-1].Index
		}
	default:
		return m, fmt.Errorf("failed to parse type %d in msgappv2 stream", typ)
	}
	return m, nil
}
