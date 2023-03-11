// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"go.uber.org/zap"

	"go.etcd.io/raft/v3/raftpb"
)

func main() {
	cluster := flag.String("cluster", "http://127.0.0.1:9021", "comma separated cluster peers")
	id := flag.Uint64("id", 1, "node ID")
	kvport := flag.Int("port", 9121, "key-value server port")
	join := flag.Bool("join", false, "join an existing cluster")
	flag.Parse()

	proposeC := make(chan string)
	defer close(proposeC)
	confChangeC := make(chan raftpb.ConfChange)
	defer close(confChangeC)

	// raft provides a commit stream for the proposals from the http api

	snapshotLogger := zap.NewExample()
	snapdir := fmt.Sprintf("raftexample-%d-snap", *id)
	snapshotStorage, err := newSnapshotStorage(snapshotLogger, snapdir)
	if err != nil {
		log.Fatalf("raftexample: %v", err)
	}

	kvs, fsm := newKVStore(proposeC)

	rc := startRaftNode(
		*id, strings.Split(*cluster, ","), *join,
		fsm, snapshotStorage,
		proposeC, confChangeC,
	)

	go func() {
		if err := rc.ProcessCommits(); err != nil {
			log.Fatalf("raftexample: %v", err)
		}
	}()

	// the key-value http handler will propose updates to raft
	serveHTTPKVAPI(kvs, *kvport, confChangeC, rc.Done())
}
