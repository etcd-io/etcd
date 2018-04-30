// Copyright 2018 The etcd Authors
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
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/raftsnap"
	"github.com/coreos/etcd/wal"
	"github.com/coreos/etcd/wal/walpb"

	"go.uber.org/zap"
)

func main() {
	snapfile := flag.String("start-snap", "", "The base name of snapshot file to start dumping")
	index := flag.Uint64("start-index", 0, "The index to start dumping")
	entrytype := flag.String("entry-type", "", `If set, filters output by entry type. Must be one or more than one of: 
	ConfigChange, Normal, Request, InternalRaftRequest, 
	IRRRange, IRRPut, IRRDeleteRange, IRRTxn, 
	IRRCompaction, IRRLeaseGrant, IRRLeaseRevoke`)

	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalf("Must provide data-dir argument (got %+v)", flag.Args())
	}
	dataDir := flag.Args()[0]

	if *snapfile != "" && *index != 0 {
		log.Fatal("start-snap and start-index flags cannot be used together.")
	}

	var (
		walsnap  walpb.Snapshot
		snapshot *raftpb.Snapshot
		err      error
	)

	isIndex := *index != 0

	if isIndex {
		fmt.Printf("Start dumping log entries from index %d.\n", *index)
		walsnap.Index = *index
	} else {
		if *snapfile == "" {
			ss := raftsnap.New(zap.NewExample(), snapDir(dataDir))
			snapshot, err = ss.Load()
		} else {
			snapshot, err = raftsnap.Read(filepath.Join(snapDir(dataDir), *snapfile))
		}

		switch err {
		case nil:
			walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
			nodes := genIDSlice(snapshot.Metadata.ConfState.Nodes)
			fmt.Printf("Snapshot:\nterm=%d index=%d nodes=%s\n",
				walsnap.Term, walsnap.Index, nodes)
		case raftsnap.ErrNoSnapshot:
			fmt.Printf("Snapshot:\nempty\n")
		default:
			log.Fatalf("Failed loading snapshot: %v", err)
		}
		fmt.Println("Start dupmping log entries from snapshot.")
	}

	w, err := wal.OpenForRead(zap.NewExample(), walDir(dataDir), walsnap)
	if err != nil {
		log.Fatalf("Failed opening WAL: %v", err)
	}
	wmetadata, state, ents, err := w.ReadAll()
	w.Close()
	if err != nil && (!isIndex || err != wal.ErrSnapshotNotFound) {
		log.Fatalf("Failed reading WAL: %v", err)
	}
	id, cid := parseWALMetadata(wmetadata)
	vid := types.ID(state.Vote)
	fmt.Printf("WAL metadata:\nnodeID=%s clusterID=%s term=%d commitIndex=%d vote=%s\n",
		id, cid, state.Term, state.Commit, vid)

	fmt.Printf("WAL entries:\n")
	fmt.Printf("lastIndex=%d\n", ents[len(ents)-1].Index)
	fmt.Printf("%4s\t%10s\ttype\tdata\n", "term", "index")
	listEntriesType(*entrytype, ents)
}

func walDir(dataDir string) string { return filepath.Join(dataDir, "member", "wal") }

func snapDir(dataDir string) string { return filepath.Join(dataDir, "member", "snap") }

func parseWALMetadata(b []byte) (id, cid types.ID) {
	var metadata etcdserverpb.Metadata
	pbutil.MustUnmarshal(&metadata, b)
	id = types.ID(metadata.NodeID)
	cid = types.ID(metadata.ClusterID)
	return id, cid
}

func genIDSlice(a []uint64) []types.ID {
	ids := make([]types.ID, len(a))
	for i, id := range a {
		ids[i] = types.ID(id)
	}
	return ids
}

// excerpt replaces middle part with ellipsis and returns a double-quoted
// string safely escaped with Go syntax.
func excerpt(str string, pre, suf int) string {
	if pre+suf > len(str) {
		return fmt.Sprintf("%q", str)
	}
	return fmt.Sprintf("%q...%q", str[:pre], str[len(str)-suf:])
}

type EntryFilter func(e raftpb.Entry) (bool, string)

// The 9 pass functions below takes the raftpb.Entry and return if the entry should be printed and the type of entry,
// the type of the entry will used in the following print function
func passConfChange(entry raftpb.Entry) (bool, string) {
	return entry.Type == raftpb.EntryConfChange, "ConfigChange"
}

func passInternalRaftRequest(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil, "InternalRaftRequest"
}

func passUnknownNormal(entry raftpb.Entry) (bool, string) {
	var rr1 etcdserverpb.Request
	var rr2 etcdserverpb.InternalRaftRequest
	return (entry.Type == raftpb.EntryNormal) && (rr1.Unmarshal(entry.Data) != nil) && (rr2.Unmarshal(entry.Data) != nil), "UnknownNormal"
}

func passIRRRange(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.Range != nil, "InternalRaftRequest"
}

func passIRRPut(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.Put != nil, "InternalRaftRequest"
}

func passIRRDeleteRange(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.DeleteRange != nil, "InternalRaftRequest"
}

func passIRRTxn(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.Txn != nil, "InternalRaftRequest"
}

func passIRRCompaction(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.Compaction != nil, "InternalRaftRequest"
}

func passIRRLeaseGrant(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.LeaseGrant != nil, "InternalRaftRequest"
}

func passIRRLeaseRevoke(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.LeaseRevoke != nil, "InternalRaftRequest"
}

func passRequest(entry raftpb.Entry) (bool, string) {
	var rr1 etcdserverpb.Request
	var rr2 etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr1.Unmarshal(entry.Data) == nil && rr2.Unmarshal(entry.Data) != nil, "Request"
}

type EntryPrinter func(e raftpb.Entry)

// The 4 print functions below print the entry format based on there types

// printInternalRaftRequest is used to print entry information for IRRRange, IRRPut,
// IRRDeleteRange and IRRTxn entries
func printInternalRaftRequest(entry raftpb.Entry) {
	var rr etcdserverpb.InternalRaftRequest
	if err := rr.Unmarshal(entry.Data); err == nil {
		fmt.Printf("%4d\t%10d\tnorm\t%s\n", entry.Term, entry.Index, rr.String())
	}
}

func printUnknownNormal(entry raftpb.Entry) {
	fmt.Printf("%4d\t%10d\tnorm\t???\n", entry.Term, entry.Index)
}

func printConfChange(entry raftpb.Entry) {
	fmt.Printf("%4d\t%10d", entry.Term, entry.Index)
	fmt.Printf("\tconf")
	var r raftpb.ConfChange
	if err := r.Unmarshal(entry.Data); err != nil {
		fmt.Printf("\t???\n")
	} else {
		fmt.Printf("\tmethod=%s id=%s\n", r.Type, types.ID(r.NodeID))
	}
}

func printRequest(entry raftpb.Entry) {
	var r etcdserverpb.Request
	if err := r.Unmarshal(entry.Data); err == nil {
		fmt.Printf("%4d\t%10d\tnorm", entry.Term, entry.Index)
		switch r.Method {
		case "":
			fmt.Printf("\tnoop\n")
		case "SYNC":
			fmt.Printf("\tmethod=SYNC time=%q\n", time.Unix(0, r.Time))
		case "QGET", "DELETE":
			fmt.Printf("\tmethod=%s path=%s\n", r.Method, excerpt(r.Path, 64, 64))
		default:
			fmt.Printf("\tmethod=%s path=%s val=%s\n", r.Method, excerpt(r.Path, 64, 64), excerpt(r.Val, 128, 0))
		}
	}
}

// evaluateEntrytypeFlag evaluates entry-type flag and choose proper filter/filters to filter entries
func evaluateEntrytypeFlag(entrytype string) []EntryFilter {
	var entrytypelist []string
	if entrytype != "" {
		entrytypelist = strings.Split(entrytype, ",")
	}

	validRequest := map[string][]EntryFilter{"ConfigChange": {passConfChange},
		"Normal":              {passInternalRaftRequest, passRequest, passUnknownNormal},
		"Request":             {passRequest},
		"InternalRaftRequest": {passInternalRaftRequest},
		"IRRRange":            {passIRRRange},
		"IRRPut":              {passIRRPut},
		"IRRDeleteRange":      {passIRRDeleteRange},
		"IRRTxn":              {passIRRTxn},
		"IRRCompaction":       {passIRRCompaction},
		"IRRLeaseGrant":       {passIRRLeaseGrant},
		"IRRLeaseRevoke":      {passIRRLeaseRevoke},
	}
	filters := make([]EntryFilter, 0)
	if len(entrytypelist) == 0 {
		filters = append(filters, passInternalRaftRequest)
		filters = append(filters, passRequest)
		filters = append(filters, passUnknownNormal)
		filters = append(filters, passConfChange)
	}
	for _, et := range entrytypelist {
		if f, ok := validRequest[et]; ok {
			filters = append(filters, f...)
		} else {
			log.Printf(`[%+v] is not a valid entry-type, ignored. 
Please set entry-type to one or more of the following: 
ConfigChange, Normal, Request, InternalRaftRequest, 
IRRRange, IRRPut, IRRDeleteRange, IRRTxn,
IRRCompaction, IRRLeaseGrant, IRRLeaseRevoke`, et)
		}
	}

	return filters
}

//  listEntriesType filters and prints entries based on the entry-type flag,
func listEntriesType(entrytype string, ents []raftpb.Entry) {
	entryFilters := evaluateEntrytypeFlag(entrytype)
	printerMap := map[string]EntryPrinter{"InternalRaftRequest": printInternalRaftRequest,
		"Request":       printRequest,
		"ConfigChange":  printConfChange,
		"UnknownNormal": printUnknownNormal}
	cnt := 0
	for _, e := range ents {
		passed := false
		currtype := ""
		for _, filter := range entryFilters {
			passed, currtype = filter(e)
			if passed {
				cnt++
				break
			}
		}
		if passed {
			printer := printerMap[currtype]
			printer(e)
		}
	}
	fmt.Printf("\nEntry types (%s) count is : %d", entrytype, cnt)
}
