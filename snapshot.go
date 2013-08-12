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

func newSnapshotConf() *snapshotConf {
	// check snapshot every 3 seconds and the threshold is 20K
	return &snapshotConf{time.Second * 3, etcdStore.TotalWrites(), 20 * 1000}
}

func monitorSnapshot() {
	for {
		time.Sleep(snapConf.checkingInterval)
		currentWrites := etcdStore.TotalWrites() - snapConf.lastWrites

		if currentWrites > snapConf.writesThr {
			raftServer.TakeSnapshot()
			snapConf.lastWrites = etcdStore.TotalWrites()
		}
	}
}
