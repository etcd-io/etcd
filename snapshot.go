/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
			r.TakeSnapshot()
			snapConf.lastWrites = etcdStore.TotalWrites()
		}
	}
}
