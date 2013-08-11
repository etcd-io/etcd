package main

import (
	"fmt"
	"time"
)

type snapshotConf struct {
	// basic
	checkingInterval time.Duration
	lastWrites       uint64
	writesThr        uint64
}

var snapConf *snapshotConf

func newSnapshotConf() *snapshotConf {
	return &snapshotConf{time.Second * 3, etcdStore.TotalWrites(), 20 * 1000}
}

func monitorSnapshot() {
	for {
		time.Sleep(snapConf.checkingInterval)
		currentWrites := etcdStore.TotalWrites() - snapConf.lastWrites

		if currentWrites > snapConf.writesThr {
			raftServer.TakeSnapshot()
			snapConf.lastWrites = etcdStore.TotalWrites()

		} else {
			fmt.Println(currentWrites)
		}
	}
}
