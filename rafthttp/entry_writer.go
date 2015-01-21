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
	"encoding/binary"
	"io"

	"github.com/coreos/etcd/pkg/metrics"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
)

type entryWriter struct {
	w         io.Writer
	id        types.ID
	ents      *metrics.Counter
	bytes     *metrics.Counter
	lastIndex *metrics.Gauge
}

func newEntryWriter(w io.Writer, id types.ID) *entryWriter {
	ew := &entryWriter{
		w:         w,
		id:        id,
		ents:      metrics.GetMap("rafthttp.stream.entries_sent").NewCounter(id.String()),
		bytes:     metrics.GetMap("rafthttp.stream.bytes_sent").NewCounter(id.String()),
		lastIndex: metrics.GetMap("rafthttp.stream.last_index_sent").NewGauge(id.String()),
	}
	return ew
}

func (ew *entryWriter) writeEntries(ents []raftpb.Entry) error {
	l := len(ents)
	if l == 0 {
		return nil
	}
	if err := binary.Write(ew.w, binary.BigEndian, uint64(l)); err != nil {
		return err
	}
	ew.bytes.AddBy(8)
	for i := 0; i < l; i++ {
		if err := ew.writeEntry(&ents[i]); err != nil {
			return err
		}
		ew.ents.Add()
	}
	ew.lastIndex.Set(int64(ents[l-1].Index))
	return nil
}

func (ew *entryWriter) writeEntry(ent *raftpb.Entry) error {
	size := ent.Size()
	if err := binary.Write(ew.w, binary.BigEndian, uint64(size)); err != nil {
		return err
	}
	b, err := ent.Marshal()
	if err != nil {
		return err
	}
	_, err = ew.w.Write(b)
	ew.bytes.AddBy(8 + int64(size))
	return err
}

func (ew *entryWriter) stop() {
	metrics.GetMap("rafthttp.stream.entries_sent").Delete(ew.id.String())
	metrics.GetMap("rafthttp.stream.bytes_sent").Delete(ew.id.String())
	metrics.GetMap("rafthttp.stream.last_index_sent").Delete(ew.id.String())
}
