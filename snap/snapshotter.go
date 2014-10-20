/*
   Copyright 2014 CoreOS, Inc.

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

package snap

import (
	"errors"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap/snappb"
)

const (
	snapSuffix = ".snap"
)

var (
	ErrNoSnapshot  = errors.New("snap: no available snapshot")
	ErrCRCMismatch = errors.New("snap: crc mismatch")
	crcTable       = crc32.MakeTable(crc32.Castagnoli)
)

type Snapshotter struct {
	dir string
}

func New(dir string) *Snapshotter {
	return &Snapshotter{
		dir: dir,
	}
}

func (s *Snapshotter) SaveSnap(snapshot raftpb.Snapshot) {
	if raft.IsEmptySnap(snapshot) {
		return
	}
	s.save(&snapshot)
}

func (s *Snapshotter) save(snapshot *raftpb.Snapshot) error {
	fname := fmt.Sprintf("%016x-%016x%s", snapshot.Term, snapshot.Index, snapSuffix)
	b, err := snapshot.Marshal()
	if err != nil {
		panic(err)
	}

	crc := crc32.Update(0, crcTable, b)
	snap := snappb.Snapshot{Crc: crc, Data: b}
	d, err := snap.Marshal()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(s.dir, fname), d, 0666)
}

func (s *Snapshotter) Load() (*raftpb.Snapshot, error) {
	names, err := s.snapNames()
	if err != nil {
		return nil, err
	}
	var snap *raftpb.Snapshot
	for _, name := range names {
		if snap, err = loadSnap(s.dir, name); err == nil {
			break
		}
	}
	return snap, err
}

func loadSnap(dir, name string) (*raftpb.Snapshot, error) {
	var err error
	var b []byte

	fpath := path.Join(dir, name)
	defer func() {
		if err != nil {
			renameBroken(fpath)
		}
	}()

	b, err = ioutil.ReadFile(fpath)
	if err != nil {
		log.Printf("Snapshotter cannot read file %v: %v", name, err)
		return nil, err
	}

	var serializedSnap snappb.Snapshot
	if err = serializedSnap.Unmarshal(b); err != nil {
		log.Printf("Corrupted snapshot file %v: %v", name, err)
		return nil, err
	}
	crc := crc32.Update(0, crcTable, serializedSnap.Data)
	if crc != serializedSnap.Crc {
		log.Printf("Corrupted snapshot file %v: crc mismatch", name)
		err = ErrCRCMismatch
		return nil, err
	}

	var snap raftpb.Snapshot
	if err = snap.Unmarshal(serializedSnap.Data); err != nil {
		log.Printf("Corrupted snapshot file %v: %v", name, err)
		return nil, err
	}
	return &snap, nil
}

// snapNames returns the filename of the snapshots in logical time order (from newest to oldest).
// If there is no avaliable snapshots, an ErrNoSnapshot will be returned.
func (s *Snapshotter) snapNames() ([]string, error) {
	dir, err := os.Open(s.dir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	snaps := checkSuffix(names)
	if len(snaps) == 0 {
		return nil, ErrNoSnapshot
	}
	sort.Sort(sort.Reverse(sort.StringSlice(snaps)))
	return snaps, nil
}

func checkSuffix(names []string) []string {
	snaps := []string{}
	for i := range names {
		if strings.HasSuffix(names[i], snapSuffix) {
			snaps = append(snaps, names[i])
		} else {
			log.Printf("Unexpected non-snap file %v", names[i])
		}
	}
	return snaps
}

func renameBroken(path string) {
	brokenPath := path + ".broken"
	if err := os.Rename(path, brokenPath); err != nil {
		log.Printf("Cannot rename broken snapshot file %v to %v: %v", path, brokenPath, err)
	}
}
