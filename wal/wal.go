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

package wal

import (
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path"
	"reflect"
	"sort"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal/walpb"
)

const (
	metadataType int64 = iota + 1
	entryType
	stateType
	crcType

	// the owner can make/remove files inside the directory
	privateDirMode = 0700
)

var (
	ErrMetadataConflict = errors.New("wal: conflicting metadata found")
	ErrFileNotFound     = errors.New("wal: file not found")
	ErrCRCMismatch      = errors.New("wal: crc mismatch")
	crcTable            = crc32.MakeTable(crc32.Castagnoli)
)

// WAL is a logical repersentation of the stable storage.
// WAL is either in read mode or append mode but not both.
// A newly created WAL is in append mode, and ready for appending records.
// A just opened WAL is in read mode, and ready for reading records.
// The WAL will be ready for appending after reading out all the previous records.
type WAL struct {
	dir      string           // the living directory of the underlay files
	metadata []byte           // metadata recorded at the head of each WAL
	state    raftpb.HardState // hardstate recorded at the head of WAL

	ri      uint64   // index of entry to start reading
	decoder *decoder // decoder to decode records

	f       *os.File // underlay file opened for appending, sync
	seq     uint64   // sequence of the wal file currently used for writes
	enti    uint64   // index of the last entry saved to the wal
	encoder *encoder // encoder to encode records
}

// Create creates a WAL ready for appending records. The given metadata is
// recorded at the head of each WAL file, and can be retrieved with ReadAll.
func Create(dirpath string, metadata []byte) (*WAL, error) {
	if Exist(dirpath) {
		return nil, os.ErrExist
	}

	if err := os.MkdirAll(dirpath, privateDirMode); err != nil {
		return nil, err
	}

	p := path.Join(dirpath, walName(0, 0))
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	w := &WAL{
		dir:      dirpath,
		metadata: metadata,
		seq:      0,
		f:        f,
		encoder:  newEncoder(f, 0),
	}
	if err := w.saveCrc(0); err != nil {
		return nil, err
	}
	if err := w.encoder.encode(&walpb.Record{Type: metadataType, Data: metadata}); err != nil {
		return nil, err
	}
	if err = w.sync(); err != nil {
		return nil, err
	}
	return w, nil
}

// OpenAtIndex opens the WAL at the given index.
// The index SHOULD have been previously committed to the WAL, or the following
// ReadAll will fail.
// The returned WAL is ready to read and the first record will be the given
// index. The WAL cannot be appended to before reading out all of its
// previous records.
func OpenAtIndex(dirpath string, index uint64) (*WAL, error) {
	names, err := fileutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	names = checkWalNames(names)
	if len(names) == 0 {
		return nil, ErrFileNotFound
	}

	sort.Sort(sort.StringSlice(names))

	nameIndex, ok := searchIndex(names, index)
	if !ok || !isValidSeq(names[nameIndex:]) {
		return nil, ErrFileNotFound
	}

	// open the wal files for reading
	rcs := make([]io.ReadCloser, 0)
	for _, name := range names[nameIndex:] {
		f, err := os.Open(path.Join(dirpath, name))
		if err != nil {
			return nil, err
		}
		rcs = append(rcs, f)
	}
	rc := MultiReadCloser(rcs...)

	// open the lastest wal file for appending
	seq, _, err := parseWalName(names[len(names)-1])
	if err != nil {
		rc.Close()
		return nil, err
	}
	last := path.Join(dirpath, names[len(names)-1])
	f, err := os.OpenFile(last, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		rc.Close()
		return nil, err
	}

	// create a WAL ready for reading
	w := &WAL{
		dir:     dirpath,
		ri:      index,
		decoder: newDecoder(rc),

		f:   f,
		seq: seq,
	}
	return w, nil
}

// ReadAll reads out all records of the current WAL.
// If it cannot read out the expected entry, it will return ErrIndexNotFound.
// After ReadAll, the WAL will be ready for appending new records.
func (w *WAL) ReadAll() (metadata []byte, state raftpb.HardState, ents []raftpb.Entry, err error) {
	rec := &walpb.Record{}
	decoder := w.decoder

	for err = decoder.decode(rec); err == nil; err = decoder.decode(rec) {
		switch rec.Type {
		case entryType:
			e := mustUnmarshalEntry(rec.Data)
			if e.Index >= w.ri {
				ents = append(ents[:e.Index-w.ri], e)
			}
			w.enti = e.Index
		case stateType:
			state = mustUnmarshalState(rec.Data)
		case metadataType:
			if metadata != nil && !reflect.DeepEqual(metadata, rec.Data) {
				state.Reset()
				return nil, state, nil, ErrMetadataConflict
			}
			metadata = rec.Data
		case crcType:
			crc := decoder.crc.Sum32()
			// current crc of decoder must match the crc of the record.
			// do no need to match 0 crc, since the decoder is a new one at this case.
			if crc != 0 && rec.Validate(crc) != nil {
				state.Reset()
				return nil, state, nil, ErrCRCMismatch
			}
			decoder.updateCRC(rec.Crc)
		default:
			state.Reset()
			return nil, state, nil, fmt.Errorf("unexpected block type %d", rec.Type)
		}
	}
	if err != io.EOF {
		state.Reset()
		return nil, state, nil, err
	}

	// close decoder, disable reading
	w.decoder.close()
	w.ri = 0

	w.metadata = metadata
	// create encoder (chain crc with the decoder), enable appending
	w.encoder = newEncoder(w.f, w.decoder.lastCRC())
	w.decoder = nil
	return metadata, state, ents, nil
}

// Cut closes current file written and creates a new one ready to append.
func (w *WAL) Cut() error {
	// create a new wal file with name sequence + 1
	fpath := path.Join(w.dir, walName(w.seq+1, w.enti+1))
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	if err = w.sync(); err != nil {
		return err
	}
	w.f.Close()

	// update writer and save the previous crc
	w.f = f
	w.seq++
	prevCrc := w.encoder.crc.Sum32()
	w.encoder = newEncoder(w.f, prevCrc)
	if err := w.saveCrc(prevCrc); err != nil {
		return err
	}
	if err := w.encoder.encode(&walpb.Record{Type: metadataType, Data: w.metadata}); err != nil {
		return err
	}
	if err := w.SaveState(&w.state); err != nil {
		return err
	}
	return w.sync()
}

func (w *WAL) sync() error {
	if w.encoder != nil {
		if err := w.encoder.flush(); err != nil {
			return err
		}
	}
	return w.f.Sync()
}

func (w *WAL) Close() error {
	if w.f != nil {
		if err := w.sync(); err != nil {
			return err
		}
		if err := w.f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (w *WAL) SaveEntry(e *raftpb.Entry) error {
	b := pbutil.MustMarshal(e)
	rec := &walpb.Record{Type: entryType, Data: b}
	if err := w.encoder.encode(rec); err != nil {
		return err
	}
	w.enti = e.Index
	return nil
}

func (w *WAL) SaveState(s *raftpb.HardState) error {
	if raft.IsEmptyHardState(*s) {
		return nil
	}
	w.state = *s
	b := pbutil.MustMarshal(s)
	rec := &walpb.Record{Type: stateType, Data: b}
	return w.encoder.encode(rec)
}

func (w *WAL) Save(st raftpb.HardState, ents []raftpb.Entry) error {
	// TODO(xiangli): no more reference operator
	if err := w.SaveState(&st); err != nil {
		return err
	}
	for i := range ents {
		if err := w.SaveEntry(&ents[i]); err != nil {
			return err
		}
	}
	return w.sync()
}

func (w *WAL) saveCrc(prevCrc uint32) error {
	return w.encoder.encode(&walpb.Record{Type: crcType, Crc: prevCrc})
}
