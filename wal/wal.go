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
	"path"
	"sort"

	"github.com/coreos/etcd/raft"
)

const (
	infoType int64 = iota + 1
	entryType
	stateType
)

var (
	ErrIdMismatch = fmt.Errorf("unmatch id")
	ErrNotFound   = fmt.Errorf("wal file is not found")
)

type WAL struct {
	f   *os.File
	bw  *bufio.Writer
	buf *bytes.Buffer
}

func newWAL(f *os.File) *WAL {
	return &WAL{f, bufio.NewWriter(f), new(bytes.Buffer)}
}

func Exist(dirpath string) bool {
	names, err := readDir(dirpath)
	if err != nil {
		return false
	}
	return len(names) != 0
}

func Create(dirpath string) (*WAL, error) {
	log.Printf("path=%s wal.create", dirpath)
	if Exist(dirpath) {
		return nil, os.ErrExist
	}
	p := path.Join(dirpath, fmt.Sprintf("%016x-%016x.wal", 0, 0))
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return newWAL(f), nil
}

func Open(dirpath string) (*WAL, error) {
	log.Printf("path=%s wal.append", dirpath)
	names, err := readDir(dirpath)
	if err != nil {
		return nil, err
	}
	names = checkWalNames(names)
	if len(names) == 0 {
		return nil, ErrNotFound
	}

	name := names[len(names)-1]
	p := path.Join(dirpath, name)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return nil, err
	}
	return newWAL(f), nil
}

// index should be the index of last log entry currently.
// Cut closes current file written and creates a new one to append.
func (w *WAL) Cut(index int64) error {
	log.Printf("path=%s wal.cut index=%d", w.f.Name(), index)
	fpath := w.f.Name()
	seq, _, err := parseWalName(path.Base(fpath))
	if err != nil {
		panic("parse correct name error")
	}
	fpath = path.Join(path.Dir(fpath), fmt.Sprintf("%016x-%016x.wal", seq+1, index))
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	w.Sync()
	w.f.Close()
	w.f = f
	w.bw = bufio.NewWriter(f)
	return nil
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

	// index of the first entry
	index int64
}

func newNode(index int64) *Node {
	return &Node{Ents: make([]raft.Entry, 0), index: index + 1}
}

func (n *Node) load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	rec := &Record{}

	err = readRecord(br, rec)
	if err != nil {
		return err
	}
	if rec.Type != infoType {
		return fmt.Errorf("the first block of wal is not infoType but %d", rec.Type)
	}
	i, err := loadInfo(rec.Data)
	if err != nil {
		return err
	}
	if n.Id != 0 && n.Id != i.Id {
		return ErrIdMismatch
	}
	n.Id = i.Id

	for err = readRecord(br, rec); err == nil; err = readRecord(br, rec) {
		switch rec.Type {
		case entryType:
			e, err := loadEntry(rec.Data)
			if err != nil {
				return err
			}
			if e.Index >= n.index {
				n.Ents = append(n.Ents[:e.Index-n.index], e)
			}
		case stateType:
			s, err := loadState(rec.Data)
			if err != nil {
				return err
			}
			n.State = s
		default:
			return fmt.Errorf("unexpected block type %d", rec.Type)
		}
	}
	if err != io.EOF {
		return err
	}
	return nil
}

func (n *Node) startFrom(index int64) error {
	diff := int(index - n.index)
	if diff > len(n.Ents) {
		return ErrNotFound
	}
	n.Ents = n.Ents[diff:]
	return nil
}

// Read loads all entries after index (index is not included).
func Read(dirpath string, index int64) (*Node, error) {
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

	_, initIndex, err := parseWalName(names[nameIndex])
	if err != nil {
		panic("parse correct name error")
	}
	n := newNode(initIndex)
	for _, name := range names[nameIndex:] {
		if err := n.load(path.Join(dirpath, name)); err != nil {
			return nil, err
		}
	}
	if err := n.startFrom(index + 1); err != nil {
		return nil, ErrNotFound
	}
	return n, nil
}

// The input names should be sorted.
// serachIndex returns the array index of the last name that has
// a smaller raft index section than the given raft index.
func searchIndex(names []string, index int64) (int, bool) {
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
		_, curIndex, err := parseWalName(name)
		if err != nil {
			panic("parse correct name error")
		}
		if index >= curIndex {
			return i, true
		}
	}
	return -1, false
}

// names should have been sorted based on sequence number.
// isValidSeq checks whether seq increases continuously.
func isValidSeq(names []string) bool {
	var lastSeq int64
	for _, name := range names {
		curSeq, _, err := parseWalName(name)
		if err != nil {
			panic("parse correct name error")
		}
		if lastSeq != 0 && lastSeq != curSeq-1 {
			return false
		}
		lastSeq = curSeq
	}
	return true
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

// readDir returns the filenames in wal directory.
func readDir(dirpath string) ([]string, error) {
	dir, err := os.Open(dirpath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return names, nil
}

func checkWalNames(names []string) []string {
	wnames := make([]string, 0)
	for _, name := range names {
		if _, _, err := parseWalName(name); err != nil {
			log.Printf("parse %s: %v", name, err)
			continue
		}
		wnames = append(wnames, name)
	}
	return wnames
}

func parseWalName(str string) (seq, index int64, err error) {
	var num int
	num, err = fmt.Sscanf(str, "%016x-%016x.wal", &seq, &index)
	if num != 2 && err == nil {
		err = fmt.Errorf("bad wal name: %s", str)
	}
	return
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
