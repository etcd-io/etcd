/*
Copyright 2014 CoreOS Inc.

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
	"log"
	"os"
	"path"
	"sort"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal/walpb"
)

const (
	infoType int64 = iota + 1
	entryType
	stateType
	crcType

	// the owner can make/remove files inside the directory
	modePrivateDir = 0700
)

var (
	ErrIdMismatch  = errors.New("wal: unmatch id")
	ErrNotFound    = errors.New("wal: file is not found")
	ErrCRCMismatch = errors.New("wal: crc mismatch")
	crcTable       = crc32.MakeTable(crc32.Castagnoli)
)

// WAL is a logical repersentation of the stable storage.
// WAL is either in read mode or append mode but not both.
// A newly created WAL is in append mode, and ready for appending records.
// A just opened WAL is in read mode, and ready for reading records.
// The WAL will be ready for appending after reading out all the previous records.
type WAL struct {
	dir string // the living directory of the underlay files

	ri      int64    // index of entry to start reading
	decoder *decoder // decoder to decode records

	f       *os.File // underlay file opened for appending, sync
	seq     int64    // current sequence of the wal file
	encoder *encoder // encoder to encode records
}

// Create creates a WAL ready for appending records.
func Create(dirpath string) (*WAL, error) {
	log.Printf("path=%s wal.create", dirpath)
	if Exist(dirpath) {
		return nil, os.ErrExist
	}

	if err := os.MkdirAll(dirpath, modePrivateDir); err != nil {
		return nil, err
	}

	p := path.Join(dirpath, fmt.Sprintf("%016x-%016x.wal", 0, 0))
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	w := &WAL{
		dir:     dirpath,
		seq:     0,
		f:       f,
		encoder: newEncoder(f, 0),
	}
	if err := w.saveCrc(0); err != nil {
		return nil, err
	}
	return w, nil
}

// OpenFromIndex opens the WAL files containing all the entries after
// the given index.
// The returned WAL is ready to read. The WAL cannot be appended to before
// reading out all of its previous records.
func OpenAtIndex(dirpath string, index int64) (*WAL, error) {
	log.Printf("path=%s wal.load index=%d", dirpath, index)
	names, err := readDir(dirpath)
	if err != nil {
		return nil, err
	}
	names = checkWalNames(names)
	if len(names) == 0 {
		return nil, ErrNotFound
	}

	sort.Sort(sort.StringSlice(names))

	nameIndex, ok := searchIndex(names, index)
	if !ok || !isValidSeq(names[nameIndex:]) {
		return nil, ErrNotFound
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
	last := path.Join(dirpath, names[len(names)-1])
	f, err := os.OpenFile(last, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		rc.Close()
		return nil, err
	}

	// create a WAL ready for reading
	w := &WAL{
		ri:      index,
		decoder: newDecoder(rc),

		f: f,
	}
	return w, nil
}

// ReadAll reads out all records of the current WAL.
// After ReadAll, the WAL will be ready for appending new records.
func (w *WAL) ReadAll() (id int64, state raftpb.State, ents []raftpb.Entry, err error) {
	rec := &walpb.Record{}
	decoder := w.decoder

	for err = decoder.decode(rec); err == nil; err = decoder.decode(rec) {
		switch rec.Type {
		case entryType:
			e := mustUnmarshalEntry(rec.Data)
			if e.Index > w.ri {
				ents = append(ents[:e.Index-w.ri-1], e)
			}
		case stateType:
			state = mustUnmarshalState(rec.Data)
		case infoType:
			i := mustUnmarshalInfo(rec.Data)
			if id != 0 && id != i.Id {
				state.Reset()
				return 0, state, nil, ErrIdMismatch
			}
			id = i.Id
		case crcType:
			crc := decoder.crc.Sum32()
			// current crc of decoder must match the crc of the record.
			// do no need to match 0 crc, since the decoder is a new one at this case.
			if crc != 0 && rec.Validate(crc) != nil {
				state.Reset()
				return 0, state, nil, ErrCRCMismatch
			}
			decoder.updateCRC(rec.Crc)
		default:
			state.Reset()
			return 0, state, nil, fmt.Errorf("unexpected block type %d", rec.Type)
		}
	}
	if err != io.EOF {
		state.Reset()
		return 0, state, nil, err
	}

	// close decoder, disable reading
	w.decoder.close()
	w.ri = 0

	// create encoder (chain crc with the decoder), enable appending
	w.encoder = newEncoder(w.f, w.decoder.lastCRC())
	w.decoder = nil
	return id, state, ents, nil
}

// index should be the index of last log entry.
// Cut closes current file written and creates a new one ready to append.
func (w *WAL) Cut(index int64) error {
	log.Printf("wal.cut index=%d", index)

	// create a new wal file with name sequence + 1
	fpath := path.Join(w.dir, fmt.Sprintf("%016x-%016x.wal", w.seq+1, index))
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	w.Sync()
	w.f.Close()

	// update writer and save the previous crc
	w.f = f
	w.seq++
	prevCrc := w.encoder.crc.Sum32()
	w.encoder = newEncoder(w.f, prevCrc)
	return w.saveCrc(prevCrc)
}

func (w *WAL) Sync() error {
	if w.encoder != nil {
		if err := w.encoder.flush(); err != nil {
			return err
		}
	}
	return w.f.Sync()
}

func (w *WAL) Close() {
	log.Printf("path=%s wal.close", w.f.Name())
	if w.f != nil {
		w.Sync()
		w.f.Close()
	}
}

func (w *WAL) SaveInfo(i *raftpb.Info) error {
	log.Printf("path=%s wal.saveInfo id=%d", w.f.Name(), i.Id)
	b, err := i.Marshal()
	if err != nil {
		panic(err)
	}
	rec := &walpb.Record{Type: infoType, Data: b}
	return w.encoder.encode(rec)
}

func (w *WAL) SaveEntry(e *raftpb.Entry) error {
	b, err := e.Marshal()
	if err != nil {
		panic(err)
	}
	rec := &walpb.Record{Type: entryType, Data: b}
	return w.encoder.encode(rec)
}

func (w *WAL) SaveState(s *raftpb.State) error {
	log.Printf("path=%s wal.saveState state=\"%+v\"", w.f.Name(), s)
	b, err := s.Marshal()
	if err != nil {
		panic(err)
	}
	rec := &walpb.Record{Type: stateType, Data: b}
	return w.encoder.encode(rec)
}

func (w *WAL) Save(st raftpb.State, ents []raftpb.Entry) {
	// TODO(xiangli): no more reference operator
	w.SaveState(&st)
	for i := range ents {
		w.SaveEntry(&ents[i])
	}
	w.Sync()
}

func (w *WAL) saveCrc(prevCrc uint32) error {
	return w.encoder.encode(&walpb.Record{Type: crcType, Crc: prevCrc})
}
