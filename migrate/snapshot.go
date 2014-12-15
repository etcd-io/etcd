package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

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
}

type sstore struct {
	Root           *node
	CurrentIndex   uint64
	CurrentVersion int
}

type node struct {
	Path string

	CreatedIndex  uint64
	ModifiedIndex uint64

	Parent *node `json:"-"` // should not encode this field! avoid circular dependency.

	ExpireTime time.Time
	ACL        string
	Value      string           // for key-value pair
	Children   map[string]*node // for directory
}

func replacePathNames(n *node, s1, s2 string) {
	n.Path = path.Clean(strings.Replace(n.Path, s1, s2, 1))
	for _, c := range n.Children {
		replacePathNames(c, s1, s2)
	}
}

func pullNodesFromEtcd(n *node) map[string]uint64 {
	out := make(map[string]uint64)
	machines := n.Children["machines"]
	for name, c := range machines.Children {
		q, err := url.ParseQuery(c.Value)
		if err != nil {
			log.Fatal("Couldn't parse old query string value")
		}
		etcdurl := q.Get("etcd")
		rafturl := q.Get("raft")

		m := generateNodeMember(name, rafturl, etcdurl)
		out[m.Name] = uint64(m.ID)
	}
	return out
}

func fixEtcd(n *node) {
	n.Path = "/0"
	machines := n.Children["machines"]
	n.Children["members"] = &node{
		Path:          "/0/members",
		CreatedIndex:  machines.CreatedIndex,
		ModifiedIndex: machines.ModifiedIndex,
		ExpireTime:    machines.ExpireTime,
		ACL:           machines.ACL,
		Children:      make(map[string]*node),
	}
	for name, c := range machines.Children {
		q, err := url.ParseQuery(c.Value)
		if err != nil {
			log.Fatal("Couldn't parse old query string value")
		}
		etcdurl := q.Get("etcd")
		rafturl := q.Get("raft")

		m := generateNodeMember(name, rafturl, etcdurl)
		attrBytes, err := json.Marshal(m.attributes)
		if err != nil {
			log.Fatal("Couldn't marshal attributes")
		}
		raftBytes, err := json.Marshal(m.raftAttributes)
		if err != nil {
			log.Fatal("Couldn't marshal raft attributes")
		}
		newNode := &node{
			Path:          path.Join("/0/members", m.ID.String()),
			CreatedIndex:  c.CreatedIndex,
			ModifiedIndex: c.ModifiedIndex,
			ExpireTime:    c.ExpireTime,
			ACL:           c.ACL,
			Children: map[string]*node{
				"attributes": &node{
					Path:          path.Join("/0/members", m.ID.String(), "attributes"),
					CreatedIndex:  c.CreatedIndex,
					ModifiedIndex: c.ModifiedIndex,
					ExpireTime:    c.ExpireTime,
					ACL:           c.ACL,
					Value:         string(attrBytes),
				},
				"raftAttributes": &node{
					Path:          path.Join("/0/members", m.ID.String(), "raftAttributes"),
					CreatedIndex:  c.CreatedIndex,
					ModifiedIndex: c.ModifiedIndex,
					ExpireTime:    c.ExpireTime,
					ACL:           c.ACL,
					Value:         string(raftBytes),
				},
			},
		}
		n.Children["members"].Children[m.ID.String()] = newNode
	}
	delete(n.Children, "machines")

}

func mangleRoot(n *node) *node {
	newRoot := &node{
		Path:          "/",
		CreatedIndex:  n.CreatedIndex,
		ModifiedIndex: n.ModifiedIndex,
		ExpireTime:    n.ExpireTime,
		ACL:           n.ACL,
		Children:      make(map[string]*node),
	}
	newRoot.Children["1"] = n
	etcd := n.Children["_etcd"]
	delete(n.Children, "_etcd")
	replacePathNames(n, "/", "/1/")
	fixEtcd(etcd)
	newRoot.Children["0"] = etcd
	return newRoot
}

func (s *Snapshot4) GetNodesFromStore() map[string]uint64 {
	st := &sstore{}
	if err := json.Unmarshal(s.State, st); err != nil {
		log.Fatal("Couldn't unmarshal snapshot")
	}
	etcd := st.Root.Children["_etcd"]
	return pullNodesFromEtcd(etcd)
}

func (s *Snapshot4) Snapshot5() *raftpb.Snapshot {
	st := &sstore{}
	if err := json.Unmarshal(s.State, st); err != nil {
		log.Fatal("Couldn't unmarshal snapshot")
	}
	st.Root = mangleRoot(st.Root)

	newState, err := json.Marshal(st)
	if err != nil {
		log.Fatal("Couldn't re-marshal new snapshot")
	}

	nodes := s.GetNodesFromStore()
	nodeList := make([]uint64, 0)
	for _, v := range nodes {
		nodeList = append(nodeList, v)
	}

	snap5 := raftpb.Snapshot{
		Data: newState,
		Metadata: raftpb.SnapshotMetadata{
			Index: s.LastIndex,
			Term:  s.LastTerm,
			ConfState: raftpb.ConfState{
				Nodes: nodeList,
			},
		},
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
			return nil, fmt.Errorf("unable to parse term from filename %q: %v", n, err)
		}

		fn.Index, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse index from filename %q: %v", n, err)
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
