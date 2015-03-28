// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wal

import (
	"io"
	"os"
	"path"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/wal/walpb"
)

// Repair tries to repair the unexpectedEOF error in the
// last wal file by truncating.
func Repair(dirpath string) bool {
	f, err := openLast(dirpath)
	if err != nil {
		return false
	}

	n := 0
	rec := &walpb.Record{}

	decoder := newDecoder(f)
	defer decoder.close()
	for {
		err := decoder.decode(rec)
		switch err {
		case nil:
			n += 8 + rec.Size()
			continue
		case io.EOF:
			return true
		case io.ErrUnexpectedEOF:
			err = f.Truncate(int64(n))
			if err != nil {
				return false
			}
			err = f.Sync()
			if err != nil {
				return false
			}
			return true
		}
	}
}

// openLast opens the last wal file for read and write.
func openLast(dirpath string) (*os.File, error) {
	names, err := fileutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	names = checkWalNames(names)
	if len(names) == 0 {
		return nil, ErrFileNotFound
	}
	last := path.Join(dirpath, names[len(names)-1])
	return os.OpenFile(last, os.O_RDWR, 0)
}
