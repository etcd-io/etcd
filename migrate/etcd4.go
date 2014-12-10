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

func snapDir5(dataDir string) string {
	return path.Join(dataDir, "snap")
}

func walDir5(dataDir string) string {
	return path.Join(dataDir, "wal")
}

func Migrate4To5(dataDir string, name string) error {
	// prep new directories
	sd5 := snapDir5(dataDir)
	if err := os.MkdirAll(sd5, 0700); err != nil {
		return fmt.Errorf("failed creating snapshot directory %s: %v", sd5, err)
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
	wd5 := walDir5(dataDir)
	w, err := wal.Create(wd5, metadata)
	if err != nil {
		return fmt.Errorf("failed initializing wal at %s: %v", wd5, err)
	}
	defer w.Close()

	// transform v0.4 data
	var snap5 *raftpb.Snapshot
	if snap4 == nil {
		log.Printf("No snapshot found")
	} else {
		log.Printf("Found snapshot: lastIndex=%d", snap4.LastIndex)

		snap5 = snap4.Snapshot5()
	}

	st5 := cfg4.HardState5()

	// If we've got the most recent snapshot, we can use it's committed index. Still likely less than the current actual index, but worth it for the replay.
	if snap5 != nil {
		st5.Commit = snap5.Metadata.Index
	}

	ents5, err := Entries4To5(ents4)
	if err != nil {
		return err
	}

	ents5Len := len(ents5)
	log.Printf("Found %d log entries: firstIndex=%d lastIndex=%d", ents5Len, ents5[0].Index, ents5[ents5Len-1].Index)

	// explicitly prepend an empty entry as the WAL code expects it
	ents5 = append(make([]raftpb.Entry, 1), ents5...)

	if err = w.Save(st5, ents5); err != nil {
		return err
	}
	log.Printf("Log migration successful")

	// migrate snapshot (if necessary) and logs
	if snap5 != nil {
		ss := snap.New(sd5)
		if err := ss.SaveSnap(*snap5); err != nil {
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
			log.Printf(p.Name)
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
