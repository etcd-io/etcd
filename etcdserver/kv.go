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

package etcdserver

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/coreos/etcd/pkg/fileutil"
	dstorage "github.com/coreos/etcd/storage"
)

const databaseFilename = "db"

type kv struct {
	// dir to host KV file and incoming snapshot file
	dir string
	ig  dstorage.ConsistentIndexGetter

	dstorage.ConsistentWatchableKV
}

func newKV(dir string, ig dstorage.ConsistentIndexGetter) (*kv, error) {
	kv := &kv{
		dir: dir,
		ig:  ig,
		ConsistentWatchableKV: dstorage.New(path.Join(dir, databaseFilename), ig),
	}
	if err := kv.ConsistentWatchableKV.Restore(); err != nil {
		kv.ConsistentWatchableKV.Close()
		return nil, fmt.Errorf("failed to restore KV (%v)", err)
	}
	return kv, nil
}

// SaveFrom saves snapshot at the given index from the given reader.
// If the snapshot with the given index has been saved successfully, it keeps
// the original saved snapshot and returns error.
// The function guarantees that SaveFrom always saves either complete
// snapshot or no snapshot, even if the call is aborted because program
// is hard killed.
func (kv *kv) SaveFrom(r io.Reader, index uint64) error {
	f, err := ioutil.TempFile(kv.dir, "tmp")
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	fn := path.Join(kv.dir, fmt.Sprintf("%016x.db", index))
	if fileutil.Exist(fn) {
		os.Remove(f.Name())
		return fmt.Errorf("snapshot to save has existed")
	}
	err = os.Rename(f.Name(), fn)
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

// restore sets the saved snapshot file at the given index
// as the underlying database, and restores KV from the snapshot.
func (kv *kv) restore(index uint64) error {
	if err := kv.ConsistentWatchableKV.Close(); err != nil {
		return fmt.Errorf("faile to close KV (%v)", err)
	}
	snapfn, err := kv.getSnapFilePath(index)
	if err != nil {
		return fmt.Errorf("failed to get snapshot file path (%v)", err)
	}
	fn := path.Join(kv.dir, databaseFilename)
	if err := os.Rename(snapfn, fn); err != nil {
		return fmt.Errorf("failed to rename snapshot file (%v)", err)
	}
	kv.ConsistentWatchableKV = dstorage.New(fn, kv.ig)
	if err := kv.ConsistentWatchableKV.Restore(); err != nil {
		return fmt.Errorf("failed to restore KV (%v)", err)
	}
	return nil
}

// getSnapFilePath returns the file path for the snapshot with given index.
// If the snapshot does not exist, it returns error.
func (kv *kv) getSnapFilePath(index uint64) (string, error) {
	fns, err := fileutil.ReadDir(kv.dir)
	if err != nil {
		return "", err
	}
	wfn := fmt.Sprintf("%016x.db", index)
	for _, fn := range fns {
		if fn == wfn {
			return path.Join(kv.dir, fn), nil
		}
	}
	return "", fmt.Errorf("snapshot file doesn't exist")
}
