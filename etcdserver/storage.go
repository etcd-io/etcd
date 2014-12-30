package etcdserver

import (
	"log"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/migrate"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/snap"
	"github.com/coreos/etcd/wal"
)

type Storage interface {
	// Save function saves ents and state to the underlying stable storage.
	// Save MUST block until st and ents are on stable storage.
	Save(st raftpb.HardState, ents []raftpb.Entry) error
	// SaveSnap function saves snapshot to the underlying stable storage.
	SaveSnap(snap raftpb.Snapshot) error

	// TODO: WAL should be able to control cut itself. After implement self-controlled cut,
	// remove it in this interface.
	// Cut cuts out a new wal file for saving new state and entries.
	Cut() error
	// Close closes the Storage and performs finalization.
	Close() error
}

type storage struct {
	*wal.WAL
	*snap.Snapshotter
}

func NewStorage(w *wal.WAL, s *snap.Snapshotter) Storage {
	return &storage{w, s}
}

// SaveSnap saves the snapshot to disk and release the locked
// wal files since they will not be used.
func (st *storage) SaveSnap(snap raftpb.Snapshot) error {
	err := st.Snapshotter.SaveSnap(snap)
	if err != nil {
		return err
	}
	err = st.WAL.ReleaseLockTo(snap.Metadata.Index)
	if err != nil {
		return err
	}
	return nil
}

func readWAL(waldir string, index uint64) (w *wal.WAL, id, cid types.ID, st raftpb.HardState, ents []raftpb.Entry) {
	var err error
	if w, err = wal.Open(waldir, index); err != nil {
		log.Fatalf("etcdserver: open wal error: %v", err)
	}
	var wmetadata []byte
	if wmetadata, st, ents, err = w.ReadAll(); err != nil {
		log.Fatalf("etcdserver: read wal error: %v", err)
	}
	var metadata pb.Metadata
	pbutil.MustUnmarshal(&metadata, wmetadata)
	id = types.ID(metadata.NodeID)
	cid = types.ID(metadata.ClusterID)
	return
}

// upgradeWAL converts an older version of the etcdServer data to the newest version.
// It must ensure that, after upgrading, the most recent version is present.
func upgradeWAL(cfg *ServerConfig, ver wal.WalVersion) error {
	if ver == wal.WALv0_4 {
		log.Print("etcdserver: converting v0.4 log to v2.0")
		err := migrate.Migrate4To2(cfg.DataDir, cfg.Name)
		if err != nil {
			log.Fatalf("etcdserver: failed migrating data-dir: %v", err)
			return err
		}
	}
	return nil
}
