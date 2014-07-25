package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
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

func (w *WAL) Close() {
	if w.f != nil {
		w.flush()
		w.f.Close()
	}
}

func (w *WAL) writeInfo(id int64) error {
	// | 8 bytes | 8 bytes |  8 bytes |
	// | type    |   len   |   nodeid |
	o, err := w.f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	if o != 0 || w.bw.Buffered() != 0 {
		return fmt.Errorf("cannot write info at %d, expect 0", o)
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
