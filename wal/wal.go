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
		w.flush()
		w.f.Close()
	}
}

func (w *WAL) writeInfo(id int64) error {
	// | 8 bytes | 8 bytes |  8 bytes |
	// | type    |   len   |   nodeid |
	if err := w.checkAtHead(); err != nil {
		return err
	}
	if err := w.writeInt64(infoType); err != nil {
		return err
	}
	if err := w.writeInt64(8); err != nil {
		return err
	}
	return w.writeInt64(id)
}

func (w *WAL) writeEntry(e *raft.Entry) error {
	// | 8 bytes | 8 bytes |  variable length |
	// | type    |   len   |   entry data     |
	if err := w.writeInt64(entryType); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	n := len(b)
	if err := w.writeInt64(int64(n)); err != nil {
		return err
	}
	if _, err := w.bw.Write(b); err != nil {
		return err
	}
	return nil
}

func (w *WAL) writeState(s *raft.State) error {
	// | 8 bytes | 8 bytes |  24 bytes |
	// | type    |   len   |   state   |
	if err := w.writeInt64(stateType); err != nil {
		return err
	}
	if err := w.writeInt64(24); err != nil {
		return err
	}
	return binary.Write(w.bw, binary.LittleEndian, s)
}

func (w *WAL) writeInt64(n int64) error {
	return binary.Write(w.bw, binary.LittleEndian, n)
}

func (w *WAL) flush() error {
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

func (w *WAL) ReadNode() (*Node, error) {
	if err := w.checkAtHead(); err != nil {
		return nil, err
	}
	br := bufio.NewReader(w.f)
	n := new(Node)

	b, err := readBlock(br)
	if err != nil {
		return nil, err
	}
	switch b.t {
	case infoType:
		id, err := parseInfo(b.d)
		if err != nil {
			return nil, err
		}
		n.Id = id
	default:
		return nil, fmt.Errorf("type = %d, want %d", b.t, infoType)
	}

	ents := make([]raft.Entry, 0)
	var state raft.State
	for {
		b, err := readBlock(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch b.t {
		case entryType:
			e, err := parseEntry(b.d)
			if err != nil {
				return nil, err
			}
			ents = append(ents, e)
		case stateType:
			s, err := parseState(b.d)
			if err != nil {
				return nil, err
			}
			state = s
		default:
			return nil, fmt.Errorf("cannot handle type %d", b.t)
		}
	}
	n.Ents = ents
	n.State = state
	return n, nil
}

func parseInfo(d []byte) (int64, error) {
	if len(d) != 8 {
		return 0, fmt.Errorf("len = %d, want 8", len(d))
	}
	buf := bytes.NewBuffer(d)
	return readInt64(buf)
}

func parseEntry(d []byte) (raft.Entry, error) {
	var e raft.Entry
	err := json.Unmarshal(d, &e)
	return e, err
}

func parseState(d []byte) (raft.State, error) {
	var s raft.State
	buf := bytes.NewBuffer(d)
	err := binary.Read(buf, binary.LittleEndian, &s)
	return s, err
}

type block struct {
	t int64
	l int64
	d []byte
}

func readBlock(r io.Reader) (*block, error) {
	typ, err := readInt64(r)
	if err != nil {
		return nil, err
	}
	l, err := readInt64(r)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	data := make([]byte, l)
	n, err := r.Read(data)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	if n != int(l) {
		return nil, fmt.Errorf("len(data) = %d, want %d", n, l)
	}
	return &block{typ, l, data}, nil
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
