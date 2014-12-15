package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"path"

	etcdserverpb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/migrate"
	"github.com/coreos/etcd/pkg/types"
	raftpb "github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/wal"
)

func walDir5(dataDir string) string {
	return path.Join(dataDir, "wal")
}

func logFile4(dataDir string) string {
	return path.Join(dataDir, "log")
}

func main() {
	version := flag.Int("version", 5, "4 or 5")
	from := flag.String("data-dir", "", "")
	flag.Parse()

	if *from == "" {
		log.Fatal("Must provide -data-dir flag")
	}

	var ents []raftpb.Entry
	var err error
	switch *version {
	case 4:
		ents, err = dump4(*from)
	case 5:
		ents, err = dump5(*from)
	default:
		err = errors.New("value of -version flag must be 4 or 5")
	}

	if err != nil {
		log.Fatalf("Failed decoding log: %v", err)
	}

	for _, e := range ents {
		msg := fmt.Sprintf("%2d %5d: ", e.Term, e.Index)
		switch e.Type {
		case raftpb.EntryNormal:
			msg = fmt.Sprintf("%s norm", msg)
			var r etcdserverpb.Request
			if err := r.Unmarshal(e.Data); err != nil {
				msg = fmt.Sprintf("%s ???", msg)
			} else {
				msg = fmt.Sprintf("%s %s %s %s", msg, r.Method, r.Path, r.Val)
			}
		case raftpb.EntryConfChange:
			msg = fmt.Sprintf("%s conf", msg)
			var r raftpb.ConfChange
			if err := r.Unmarshal(e.Data); err != nil {
				msg = fmt.Sprintf("%s ???", msg)
			} else {
				msg = fmt.Sprintf("%s %s %s %s", msg, r.Type, types.ID(r.NodeID), r.Context)
			}
		}
		fmt.Println(msg)
	}
}

func dump4(dataDir string) ([]raftpb.Entry, error) {
	lf4 := logFile4(dataDir)
	ents, err := migrate.DecodeLog4FromFile(lf4)
	if err != nil {
		return nil, err
	}

	return migrate.Entries4To5(ents)
}

func dump5(dataDir string) ([]raftpb.Entry, error) {
	wd5 := walDir5(dataDir)
	if !wal.Exist(wd5) {
		return nil, fmt.Errorf("No wal exists at %s", wd5)
	}

	w, err := wal.OpenAtIndex(wd5, 0)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	_, _, ents, err := w.ReadAll()
	return ents, err
}
