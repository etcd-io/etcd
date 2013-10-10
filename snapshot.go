package main

import (
	"time"
)

// basic conf.
// TODO: find a good policy to do snapshot
type snapshotConf struct {
	// Etcd will check if snapshot is need every checkingInterval
	checkingInterval time.Duration
	// The number of writes when the last snapshot happened
	lastWrites uint64
	// If the incremental number of writes since the last snapshot
	// exceeds the write Threshold, etcd will do a snapshot
	writesThr uint64
}

var snapConf *snapshotConf

func (r *raftServer) newSnapshotConf() *snapshotConf {
	// check snapshot every 3 seconds and the threshold is 20K
	return &snapshotConf{time.Second * 3, 0, 20 * 1000}
}

func (r *raftServer) monitorSnapshot() {
	for {
		time.Sleep(snapConf.checkingInterval)
		//currentWrites := etcdStore.TotalWrites() - snapConf.lastWrites
		currentWrites := 0
		if uint64(currentWrites) > snapConf.writesThr {
			r.TakeSnapshot()
			snapConf.lastWrites = 0
		}
	}
}
