package migrate

import (
	"fmt"
	"log"
	"os"
	"path"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	raftpb "github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
)

func snapDir4(dataDir string) string {
	return path.Join(dataDir, "snapshot")
}

func logFile4(dataDir string) string {
	return path.Join(dataDir, "log")
}

func cfgFile4(dataDir string) string {
	return path.Join(dataDir, "conf")
}

func snapDir2(dataDir string) string {
	return path.Join(dataDir, "snap")
}

func walDir2(dataDir string) string {
	return path.Join(dataDir, "wal")
}

func Migrate4To2(dataDir string, name string) error {
	// prep new directories
	sd2 := snapDir2(dataDir)
	if err := os.MkdirAll(sd2, 0700); err != nil {
		return fmt.Errorf("failed creating snapshot directory %s: %v", sd2, err)
	}

	// read v0.4 data
	snap4, err := DecodeLatestSnapshot4FromDir(snapDir4(dataDir))
	if err != nil {
		return err
	}

	cfg4, err := DecodeConfig4FromFile(cfgFile4(dataDir))
	if err != nil {
		return err
	}

	ents4, err := DecodeLog4FromFile(logFile4(dataDir))
	if err != nil {
		return err
	}

	nodeIDs := ents4.NodeIDs()
	nodeID := GuessNodeID(nodeIDs, snap4, cfg4, name)

	if nodeID == 0 {
		return fmt.Errorf("Couldn't figure out the node ID from the log or flags, cannot convert")
	}

	metadata := pbutil.MustMarshal(&pb.Metadata{NodeID: nodeID, ClusterID: 0x04add5})
	wd2 := walDir2(dataDir)
	w, err := wal.Create(wd2, metadata)
	if err != nil {
		return fmt.Errorf("failed initializing wal at %s: %v", wd2, err)
	}
	defer w.Close()

	// transform v0.4 data
	var snap2 *raftpb.Snapshot
	if snap4 == nil {
		log.Printf("No snapshot found")
	} else {
		log.Printf("Found snapshot: lastIndex=%d", snap4.LastIndex)

		snap2 = snap4.Snapshot2()
	}

	st2 := cfg4.HardState2()

	// If we've got the most recent snapshot, we can use it's committed index. Still likely less than the current actual index, but worth it for the replay.
	if snap2 != nil {
		st2.Commit = snap2.Metadata.Index
	}

	ents2, err := Entries4To2(ents4)
	if err != nil {
		return err
	}

	ents2Len := len(ents2)
	log.Printf("Found %d log entries: firstIndex=%d lastIndex=%d", ents2Len, ents2[0].Index, ents2[ents2Len-1].Index)

	// explicitly prepend an empty entry as the WAL code expects it
	ents2 = append(make([]raftpb.Entry, 1), ents2...)

	if err = w.Save(st2, ents2); err != nil {
		return err
	}
	log.Printf("Log migration successful")

	// migrate snapshot (if necessary) and logs
	if snap2 != nil {
		ss := snap.New(sd2)
		if err := ss.SaveSnap(*snap2); err != nil {
			return err
		}
		log.Printf("Snapshot migration successful")
	}

	return nil
}

func GuessNodeID(nodes map[string]uint64, snap4 *Snapshot4, cfg *Config4, name string) uint64 {
	var snapNodes map[string]uint64
	if snap4 != nil {
		snapNodes = snap4.GetNodesFromStore()
	}
	// First, use the flag, if set.
	if name != "" {
		log.Printf("Using suggested name %s", name)
		if val, ok := nodes[name]; ok {
			log.Printf("Assigning %s the ID %s", name, types.ID(val))
			return val
		}
		if snapNodes != nil {
			if val, ok := snapNodes[name]; ok {
				log.Printf("Assigning %s the ID %s", name, types.ID(val))
				return val
			}
		}
		log.Printf("Name not found, autodetecting...")
	}
	// Next, look at the snapshot peers, if that exists.
	if snap4 != nil {
		//snapNodes := make(map[string]uint64)
		//for _, p := range snap4.Peers {
		//m := generateNodeMember(p.Name, p.ConnectionString, "")
		//snapNodes[p.Name] = uint64(m.ID)
		//}
		for _, p := range cfg.Peers {
			delete(snapNodes, p.Name)
		}
		if len(snapNodes) == 1 {
			for name, id := range nodes {
				log.Printf("Autodetected from snapshot: name %s", name)
				return id
			}
		}
	}
	// Then, try and deduce from the log.
	for _, p := range cfg.Peers {
		delete(nodes, p.Name)
	}
	if len(nodes) == 1 {
		for name, id := range nodes {
			log.Printf("Autodetected name %s", name)
			return id
		}
	}
	return 0
}
