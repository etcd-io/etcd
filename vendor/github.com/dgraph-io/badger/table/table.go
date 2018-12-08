/*
 * Copyright 2017 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package table

import (
	"encoding/binary"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/AndreasBriese/bbloom"
	"github.com/dgraph-io/badger/options"
	"github.com/dgraph-io/badger/y"
	"github.com/pkg/errors"
)

const fileSuffix = ".sst"

type keyOffset struct {
	key    []byte
	offset int
	len    int
}

// Table represents a loaded table file with the info we have about it
type Table struct {
	sync.Mutex

	fd        *os.File // Own fd.
	tableSize int      // Initialized in OpenTable, using fd.Stat().

	blockIndex []keyOffset
	ref        int32 // For file garbage collection.  Atomic.

	loadingMode options.FileLoadingMode
	mmap        []byte // Memory mapped.

	// The following are initialized once and const.
	smallest, biggest []byte // Smallest and largest keys.
	id                uint64 // file id, part of filename

	bf bbloom.Bloom
}

// IncrRef increments the refcount (having to do with whether the file should be deleted)
func (t *Table) IncrRef() {
	atomic.AddInt32(&t.ref, 1)
}

// DecrRef decrements the refcount and possibly deletes the table
func (t *Table) DecrRef() error {
	newRef := atomic.AddInt32(&t.ref, -1)
	if newRef == 0 {
		// We can safely delete this file, because for all the current files, we always have
		// at least one reference pointing to them.

		// It's necessary to delete windows files
		if t.loadingMode == options.MemoryMap {
			y.Munmap(t.mmap)
		}
		if err := t.fd.Truncate(0); err != nil {
			// This is very important to let the FS know that the file is deleted.
			return err
		}
		filename := t.fd.Name()
		if err := t.fd.Close(); err != nil {
			return err
		}
		if err := os.Remove(filename); err != nil {
			return err
		}
	}
	return nil
}

type block struct {
	offset int
	data   []byte
}

func (b block) NewIterator() *blockIterator {
	return &blockIterator{data: b.data}
}

// OpenTable assumes file has only one table and opens it.  Takes ownership of fd upon function
// entry.  Returns a table with one reference count on it (decrementing which may delete the file!
// -- consider t.Close() instead).  The fd has to writeable because we call Truncate on it before
// deleting.
func OpenTable(fd *os.File, loadingMode options.FileLoadingMode) (*Table, error) {
	fileInfo, err := fd.Stat()
	if err != nil {
		// It's OK to ignore fd.Close() errs in this function because we have only read
		// from the file.
		_ = fd.Close()
		return nil, y.Wrap(err)
	}

	filename := fileInfo.Name()
	id, ok := ParseFileID(filename)
	if !ok {
		_ = fd.Close()
		return nil, errors.Errorf("Invalid filename: %s", filename)
	}
	t := &Table{
		fd:          fd,
		ref:         1, // Caller is given one reference.
		id:          id,
		loadingMode: loadingMode,
	}

	t.tableSize = int(fileInfo.Size())

	if loadingMode == options.MemoryMap {
		t.mmap, err = y.Mmap(fd, false, fileInfo.Size())
		if err != nil {
			_ = fd.Close()
			return nil, y.Wrapf(err, "Unable to map file")
		}
	} else if loadingMode == options.LoadToRAM {
		err = t.loadToRAM()
		if err != nil {
			_ = fd.Close()
			return nil, y.Wrap(err)
		}
	}

	if err := t.readIndex(); err != nil {
		return nil, y.Wrap(err)
	}

	it := t.NewIterator(false)
	defer it.Close()
	it.Rewind()
	if it.Valid() {
		t.smallest = it.Key()
	}

	it2 := t.NewIterator(true)
	defer it2.Close()
	it2.Rewind()
	if it2.Valid() {
		t.biggest = it2.Key()
	}
	return t, nil
}

// Close closes the open table.  (Releases resources back to the OS.)
func (t *Table) Close() error {
	if t.loadingMode == options.MemoryMap {
		y.Munmap(t.mmap)
	}

	return t.fd.Close()
}

func (t *Table) read(off int, sz int) ([]byte, error) {
	if len(t.mmap) > 0 {
		if len(t.mmap[off:]) < sz {
			return nil, y.ErrEOF
		}
		return t.mmap[off : off+sz], nil
	}

	res := make([]byte, sz)
	nbr, err := t.fd.ReadAt(res, int64(off))
	y.NumReads.Add(1)
	y.NumBytesRead.Add(int64(nbr))
	return res, err
}

func (t *Table) readNoFail(off int, sz int) []byte {
	res, err := t.read(off, sz)
	y.Check(err)
	return res
}

func (t *Table) readIndex() error {
	readPos := t.tableSize

	// Read bloom filter.
	readPos -= 4
	buf := t.readNoFail(readPos, 4)
	bloomLen := int(binary.BigEndian.Uint32(buf))
	readPos -= bloomLen
	data := t.readNoFail(readPos, bloomLen)
	t.bf = bbloom.JSONUnmarshal(data)

	readPos -= 4
	buf = t.readNoFail(readPos, 4)
	restartsLen := int(binary.BigEndian.Uint32(buf))

	readPos -= 4 * restartsLen
	buf = t.readNoFail(readPos, 4*restartsLen)

	offsets := make([]int, restartsLen)
	for i := 0; i < restartsLen; i++ {
		offsets[i] = int(binary.BigEndian.Uint32(buf[:4]))
		buf = buf[4:]
	}

	// The last offset stores the end of the last block.
	for i := 0; i < len(offsets); i++ {
		var o int
		if i == 0 {
			o = 0
		} else {
			o = offsets[i-1]
		}

		ko := keyOffset{
			offset: o,
			len:    offsets[i] - o,
		}
		t.blockIndex = append(t.blockIndex, ko)
	}

	che := make(chan error, len(t.blockIndex))
	blocks := make(chan int, len(t.blockIndex))

	for i := 0; i < len(t.blockIndex); i++ {
		blocks <- i
	}

	for i := 0; i < 64; i++ { // Run 64 goroutines.
		go func() {
			var h header

			for index := range blocks {
				ko := &t.blockIndex[index]

				offset := ko.offset
				buf, err := t.read(offset, h.Size())
				if err != nil {
					che <- errors.Wrap(err, "While reading first header in block")
					continue
				}

				h.Decode(buf)
				y.AssertTruef(h.plen == 0, "Key offset: %+v, h.plen = %d", *ko, h.plen)

				offset += h.Size()
				buf = make([]byte, h.klen)
				var out []byte
				if out, err = t.read(offset, int(h.klen)); err != nil {
					che <- errors.Wrap(err, "While reading first key in block")
					continue
				}
				y.AssertTrue(len(buf) == copy(buf, out))

				ko.key = buf
				che <- nil
			}
		}()
	}
	close(blocks) // to stop reading goroutines

	var readError error
	for i := 0; i < len(t.blockIndex); i++ {
		if err := <-che; err != nil && readError == nil {
			readError = err
		}
	}
	if readError != nil {
		return readError
	}

	return nil
}

func (t *Table) block(idx int) (block, error) {
	y.AssertTruef(idx >= 0, "idx=%d", idx)
	if idx >= len(t.blockIndex) {
		return block{}, errors.New("block out of index")
	}

	ko := t.blockIndex[idx]
	blk := block{
		offset: ko.offset,
	}
	var err error
	blk.data, err = t.read(blk.offset, ko.len)
	return blk, err
}

// Size is its file size in bytes
func (t *Table) Size() int64 { return int64(t.tableSize) }

// Smallest is its smallest key, or nil if there are none
func (t *Table) Smallest() []byte { return t.smallest }

// Biggest is its biggest key, or nil if there are none
func (t *Table) Biggest() []byte { return t.biggest }

// Filename is NOT the file name.  Just kidding, it is.
func (t *Table) Filename() string { return t.fd.Name() }

// ID is the table's ID number (used to make the file name).
func (t *Table) ID() uint64 { return t.id }

// DoesNotHave returns true if (but not "only if") the table does not have the key.  It does a
// bloom filter lookup.
func (t *Table) DoesNotHave(key []byte) bool { return !t.bf.Has(key) }

// ParseFileID reads the file id out of a filename.
func ParseFileID(name string) (uint64, bool) {
	name = path.Base(name)
	if !strings.HasSuffix(name, fileSuffix) {
		return 0, false
	}
	//	suffix := name[len(fileSuffix):]
	name = strings.TrimSuffix(name, fileSuffix)
	id, err := strconv.Atoi(name)
	if err != nil {
		return 0, false
	}
	y.AssertTrue(id >= 0)
	return uint64(id), true
}

// IDToFilename does the inverse of ParseFileID
func IDToFilename(id uint64) string {
	return fmt.Sprintf("%06d", id) + fileSuffix
}

// NewFilename should be named TableFilepath -- it combines the dir with the ID to make a table
// filepath.
func NewFilename(id uint64, dir string) string {
	return filepath.Join(dir, IDToFilename(id))
}

func (t *Table) loadToRAM() error {
	t.mmap = make([]byte, t.tableSize)
	read, err := t.fd.ReadAt(t.mmap, 0)
	if err != nil || read != t.tableSize {
		return y.Wrapf(err, "Unable to load file in memory. Table file: %s", t.Filename())
	}
	y.NumReads.Add(1)
	y.NumBytesRead.Add(int64(read))
	return nil
}
