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
	"encoding/json"
	"fmt"
	"io"
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
	f, err := os.Open(path)
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
	if w.f != nil {
		w.Sync()
		w.f.Close()
	}
}

func (w *WAL) SaveInfo(id int64) error {
	if err := w.checkAtHead(); err != nil {
		return err
	}
	w.buf.Reset()
	err := binary.Write(w.buf, binary.LittleEndian, id)
	if err != nil {
		panic(err)
	}
	return writeBlock(w.bw, infoType, w.buf.Bytes())
}

func (w *WAL) SaveEntry(e *raft.Entry) error {
	// protobuf?
	b, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return writeBlock(w.bw, entryType, b)
}

func (w *WAL) SaveState(s *raft.State) error {
	w.buf.Reset()
	err := binary.Write(w.buf, binary.LittleEndian, s)
	if err != nil {
		panic(err)
	}
	return writeBlock(w.bw, stateType, w.buf.Bytes())
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
	if err := w.checkAtHead(); err != nil {
		return nil, err
	}
	br := bufio.NewReader(w.f)
	b := &block{}

	err := readBlock(br, b)
	if err != nil {
		return nil, err
	}
	if b.t != infoType {
		return nil, fmt.Errorf("the first block of wal is not infoType but %d", b.t)
	}
	id, err := loadInfo(b.d)
	if err != nil {
		return nil, err
	}

	ents := make([]raft.Entry, 0)
	var state raft.State
	for err = readBlock(br, b); err == nil; err = readBlock(br, b) {
		switch b.t {
		case entryType:
			e, err := loadEntry(b.d)
			if err != nil {
				return nil, err
			}
			ents = append(ents[:e.Index-1], e)
		case stateType:
			s, err := loadState(b.d)
			if err != nil {
				return nil, err
			}
			state = s
		default:
			return nil, fmt.Errorf("unexpected block type %d", b.t)
		}
	}
	if err != io.EOF {
		return nil, err
	}
	return &Node{id, ents, state}, nil
}

func loadInfo(d []byte) (int64, error) {
	if len(d) != 8 {
		return 0, fmt.Errorf("len = %d, want 8", len(d))
	}
	buf := bytes.NewBuffer(d)
	return readInt64(buf)
}

func loadEntry(d []byte) (raft.Entry, error) {
	var e raft.Entry
	err := json.Unmarshal(d, &e)
	return e, err
}

func loadState(d []byte) (raft.State, error) {
	var s raft.State
	buf := bytes.NewBuffer(d)
	err := binary.Read(buf, binary.LittleEndian, &s)
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
