package snap

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"

	"github.com/coreos/etcd/raft"
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

func (s *Snapshotter) Save(snapshot *raft.Snapshot) error {
	n := fmt.Sprintf("%016x%016x%016x.snap", snapshot.ClusterId, snapshot.Term, snapshot.Index)
	// TODO(xiangli): make raft.Snapshot a protobuf type
	b, err := json.Marshal(snapshot)
	if err != nil {
		panic(err)
	}
	crc := crc32.Update(0, crcTable, b)
	snap := Snapshot{Crc: crc, Data: b}
	d, err := snap.Marshal()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(s.dir, n), d, 0666)
}

func (s *Snapshotter) Load() (*raft.Snapshot, error) {
	dir, err := os.Open(s.dir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, ErrNoSnapshot
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	var snap raft.Snapshot
	var serializedSnap Snapshot
	var b []byte
	for _, name := range names {
		b, err = ioutil.ReadFile(path.Join(s.dir, name))
		if err != nil {
			log.Printf("Snapshotter cannot read file %v: %v", name, err)
			continue
		}
		if err = serializedSnap.Unmarshal(b); err != nil {
			log.Printf("Corruptted snapshot file %v: %v", name, err)
			continue
		}
		crc := crc32.Update(0, crcTable, serializedSnap.Data)
		if crc != serializedSnap.Crc {
			log.Printf("Corruptted snapshot file %v: crc mismatch", name)
			err = ErrCRCMismatch
			continue
		}
		if err = json.Unmarshal(serializedSnap.Data, &snap); err != nil {
			log.Printf("Corruptted snapshot file %v: %v", name, err)
		}
		break
	}
	if err != nil {
		return nil, err
	}
	return &snap, nil
}
