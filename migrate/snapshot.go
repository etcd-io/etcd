package migrate

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
	"strconv"
	"strings"

	raftpb "github.com/coreos/etcd/raft/raftpb"
)

type Snapshot4 struct {
	State     []byte `json:"state"`
	LastIndex uint64 `json:"lastIndex"`
	LastTerm  uint64 `json:"lastTerm"`

	Peers []struct {
		Name             string `json:"name"`
		ConnectionString string `json:"connectionString"`
	} `json:"peers"`

	//TODO(bcwaldon): is this needed?
	//Path  string `json:"path"`
}

func (s *Snapshot4) Snapshot5() *raftpb.Snapshot {
	snap5 := raftpb.Snapshot{
		Data:  s.State,
		Index: int64(s.LastIndex),
		Term:  int64(s.LastTerm),
		Nodes: make([]int64, len(s.Peers)),
	}

	for i, p := range s.Peers {
		snap5.Nodes[i] = hashName(p.Name)
	}

	return &snap5
}

func DecodeLatestSnapshot4FromDir(snapdir string) (*Snapshot4, error) {
	fname, err := FindLatestFile(snapdir)
	if err != nil {
		return nil, err
	}

	if fname == "" {
		return nil, nil
	}

	snappath := path.Join(snapdir, fname)
	log.Printf("Decoding snapshot from %s", snappath)

	return DecodeSnapshot4FromFile(snappath)
}

// FindLatestFile identifies the "latest" filename in a given directory
// by sorting all the files and choosing the highest value.
func FindLatestFile(dirpath string) (string, error) {
	dir, err := os.OpenFile(dirpath, os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return "", err
	}
	defer dir.Close()

	fnames, err := dir.Readdirnames(-1)
	if err != nil {
		return "", err
	}

	if len(fnames) == 0 {
		return "", nil
	}

	names, err := NewSnapshotFileNames(fnames)
	if err != nil {
		return "", err
	}

	return names[len(names)-1].FileName, nil
}

func DecodeSnapshot4FromFile(path string) (*Snapshot4, error) {
	// Read snapshot data.
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return DecodeSnapshot4(f)
}

func DecodeSnapshot4(f *os.File) (*Snapshot4, error) {
	// Verify checksum
	var checksum uint32
	n, err := fmt.Fscanf(f, "%08x\n", &checksum)
	if err != nil {
		return nil, err
	} else if n != 1 {
		return nil, errors.New("miss heading checksum")
	}

	// Load remaining snapshot contents.
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// Generate checksum.
	byteChecksum := crc32.ChecksumIEEE(b)
	if uint32(checksum) != byteChecksum {
		return nil, errors.New("bad checksum")
	}

	// Decode snapshot.
	snapshot := new(Snapshot4)
	if err = json.Unmarshal(b, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func NewSnapshotFileNames(names []string) ([]SnapshotFileName, error) {
	s := make([]SnapshotFileName, 0)
	for _, n := range names {
		trimmed := strings.TrimSuffix(n, ".ss")
		if trimmed == n {
			return nil, fmt.Errorf("file %q does not have .ss extension", n)
		}

		parts := strings.SplitN(trimmed, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unrecognized file name format %q", n)
		}

		fn := SnapshotFileName{FileName: n}

		var err error
		fn.Term, err = strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse term from filename %q: %v", err)
		}

		fn.Index, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse index from filename %q: %v", err)
		}

		s = append(s, fn)
	}

	sortable := SnapshotFileNames(s)
	sort.Sort(&sortable)
	return s, nil
}

type SnapshotFileNames []SnapshotFileName
type SnapshotFileName struct {
	FileName string
	Term     uint64
	Index    uint64
}

func (n *SnapshotFileNames) Less(i, j int) bool {
	iTerm, iIndex := (*n)[i].Term, (*n)[i].Index
	jTerm, jIndex := (*n)[j].Term, (*n)[j].Index
	return iTerm < jTerm || (iTerm == jTerm && iIndex < jIndex)
}

func (n *SnapshotFileNames) Swap(i, j int) {
	(*n)[i], (*n)[j] = (*n)[j], (*n)[i]
}

func (n *SnapshotFileNames) Len() int {
	return len([]SnapshotFileName(*n))
}
