package migrate

import (
	"fmt"
	"log"
	"os"
	"path"

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

func Migrate4To5(dataDir string) error {
	// prep new directories
	sd5 := snapDir5(dataDir)
	if err := os.MkdirAll(sd5, 0700); err != nil {
		return fmt.Errorf("failed creating snapshot directory %s: %v", sd5, err)
	}

	wd5 := walDir5(dataDir)
	w, err := wal.Create(wd5)
	if err != nil {
		return fmt.Errorf("failed initializing wal at %s: %v", wd5, err)
	}
	defer w.Close()

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

	// transform v0.4 data
	var snap5 *raftpb.Snapshot
	if snap4 == nil {
		log.Printf("No snapshot found")
	} else {
		log.Printf("Found snapshot: lastIndex=%d", snap4.LastIndex)

		snap5 = snap4.Snapshot5()
	}

	st5 := cfg4.HardState5()

	ents5, err := Entries4To5(uint64(st5.Commit), ents4)
	if err != nil {
		return err
	}

	ents5Len := len(ents5)
	log.Printf("Found %d log entries: firstIndex=%d lastIndex=%d", ents5Len, ents5[0].Index, ents5[ents5Len-1].Index)

	// migrate snapshot (if necessary) and logs
	if snap5 != nil {
		ss := snap.New(sd5)
		ss.SaveSnap(*snap5)
		log.Printf("Snapshot migration successful")
	}

	// explicitly prepend an empty entry as the WAL code expects it
	ents5 = append(make([]raftpb.Entry, 1), ents5...)

	w.Save(st5, ents5)
	log.Printf("Log migration successful")

	return nil
}
