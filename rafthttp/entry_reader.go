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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codahale/metrics"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

type entryReader struct {
	r  io.Reader
	id types.ID
}

func newEntryReader(r io.Reader, id types.ID) *entryReader {
	return &entryReader{
		r:  r,
		id: id,
	}
}

func (er *entryReader) readEntries() ([]raftpb.Entry, error) {
	var l uint64
	if err := binary.Read(er.r, binary.BigEndian, &l); err != nil {
		return nil, err
	}
	metrics.Counter("rafthttp.stream.bytes_received." + er.id.String()).AddN(8)
	ents := make([]raftpb.Entry, int(l))
	for i := 0; i < int(l); i++ {
		if err := er.readEntry(&ents[i]); err != nil {
			return nil, err
		}
		metrics.Counter("rafthttp.stream.entries_received." + er.id.String()).AddN(8)
	}
	if l > 0 {
		metrics.Gauge("rafthttp.stream.last_index_received." + er.id.String()).Set(int64(ents[l-1].Index))
	}
	return ents, nil
}

func (er *entryReader) readEntry(ent *raftpb.Entry) error {
	var l uint64
	if err := binary.Read(er.r, binary.BigEndian, &l); err != nil {
		return err
	}
	buf := make([]byte, int(l))
	if _, err := io.ReadFull(er.r, buf); err != nil {
		return err
	}
	metrics.Counter("rafthttp.stream.bytes_received." + er.id.String()).AddN(8 + uint64(l))
	return ent.Unmarshal(buf)
}

func (er *entryReader) stop() {
	metrics.Counter("rafthttp.stream.bytes_received." + er.id.String()).Remove()
	metrics.Counter("rafthttp.stream.entries_received." + er.id.String()).Remove()
	metrics.Gauge("rafthttp.stream.last_index_received." + er.id.String()).Remove()
}
