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
	f  *os.File
	bw *bufio.Writer
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
	bw := bufio.NewWriter(f)
	return &WAL{f, bw}, nil
}

func Open(path string) (*WAL, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	bw := bufio.NewWriter(f)
	return &WAL{f, bw}, nil
}

func (w *WAL) Close() {
	if w.f != nil {
		w.Flush()
		w.f.Close()
	}
}

func (w *WAL) SaveInfo(id int64) error {
	if err := w.checkAtHead(); err != nil {
		return err
	}
	// cache the buffer?
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, id)
	if err != nil {
		panic(err)
	}
	return writeBlock(w.bw, infoType, buf.Bytes())
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
	// cache the buffer?
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, s)
	if err != nil {
		panic(err)
	}
	return writeBlock(w.bw, stateType, buf.Bytes())
}

func (w *WAL) Flush() error {
	return w.bw.Flush()
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

	b, err := readBlock(br)
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
	for b, err = readBlock(br); err == nil; b, err = readBlock(br) {
		switch b.t {
		case entryType:
			e, err := loadEntry(b.d)
			if err != nil {
				return nil, err
			}
			ents = append(ents, e)
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

func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
