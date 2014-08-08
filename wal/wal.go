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
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/coreos/etcd/raft"
)

var (
	infoType  = int64(1)
	entryType = int64(2)
	stateType = int64(3)
)

type WAL struct {
	f   *os.File
	bw  *bufio.Writer
	buf *bytes.Buffer
}

func newWAL(f *os.File) *WAL {
	return &WAL{f, bufio.NewWriter(f), new(bytes.Buffer)}
}

func New(path string) (*WAL, error) {
	log.Printf("path=%s wal.new", path)
	f, err := os.Open(path)
	if err == nil {
		f.Close()
		return nil, os.ErrExist
	}
	f, err = os.Create(path)
	if err != nil {
		return nil, err
	}
	return newWAL(f), nil
}

func Open(path string) (*WAL, error) {
	log.Printf("path=%s wal.open", path)
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return newWAL(f), nil
}

func (w *WAL) Sync() error {
	if err := w.bw.Flush(); err != nil {
		return err
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

func (w *WAL) SaveInfo(i *raft.Info) error {
	log.Printf("path=%s wal.saveInfo id=%d", w.f.Name(), i.Id)
	if err := w.checkAtHead(); err != nil {
		return err
	}
	b, err := i.Marshal()
	if err != nil {
		panic(err)
	}
	rec := &Record{Type: infoType, Data: b}
	return writeRecord(w.bw, rec)
}

func (w *WAL) SaveEntry(e *raft.Entry) error {
	b, err := e.Marshal()
	if err != nil {
		panic(err)
	}
	rec := &Record{Type: entryType, Data: b}
	return writeRecord(w.bw, rec)
}

func (w *WAL) SaveState(s *raft.State) error {
	log.Printf("path=%s wal.saveState state=\"%+v\"", w.f.Name(), s)
	b, err := s.Marshal()
	if err != nil {
		panic(err)
	}
	rec := &Record{Type: stateType, Data: b}
	return writeRecord(w.bw, rec)
}

func (w *WAL) checkAtHead() error {
	o, err := w.f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	if o != 0 || w.bw.Buffered() != 0 {
		return fmt.Errorf("cannot write info at %d, expect 0", max(o, int64(w.bw.Buffered())))
	}
	return nil
}

type Node struct {
	Id    int64
	Ents  []raft.Entry
	State raft.State
}

func (w *WAL) LoadNode() (*Node, error) {
	log.Printf("path=%s wal.loadNode", w.f.Name())
	if err := w.checkAtHead(); err != nil {
		return nil, err
	}
	br := bufio.NewReader(w.f)
	rec := &Record{}

	err := readRecord(br, rec)
	if err != nil {
		return nil, err
	}
	if rec.Type != infoType {
		return nil, fmt.Errorf("the first block of wal is not infoType but %d", rec.Type)
	}
	i, err := loadInfo(rec.Data)
	if err != nil {
		return nil, err
	}

	ents := make([]raft.Entry, 0)
	var state raft.State
	for err = readRecord(br, rec); err == nil; err = readRecord(br, rec) {
		switch rec.Type {
		case entryType:
			e, err := loadEntry(rec.Data)
			if err != nil {
				return nil, err
			}
			ents = append(ents[:e.Index-1], e)
		case stateType:
			s, err := loadState(rec.Data)
			if err != nil {
				return nil, err
			}
			state = s
		default:
			return nil, fmt.Errorf("unexpected block type %d", rec.Type)
		}
	}
	if err != io.EOF {
		return nil, err
	}
	return &Node{i.Id, ents, state}, nil
}

func loadInfo(d []byte) (raft.Info, error) {
	var i raft.Info
	err := i.Unmarshal(d)
	if err != nil {
		panic(err)
	}
	return i, err
}

func loadEntry(d []byte) (raft.Entry, error) {
	var e raft.Entry
	err := e.Unmarshal(d)
	if err != nil {
		panic(err)
	}
	return e, err
}

func loadState(d []byte) (raft.State, error) {
	var s raft.State
	err := s.Unmarshal(d)
	return s, err
}

func writeInt64(w io.Writer, n int64) error {
	return binary.Write(w, binary.LittleEndian, n)
}

func readInt64(r io.Reader) (int64, error) {
	var n int64
	err := binary.Read(r, binary.LittleEndian, &n)
	return n, err
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
