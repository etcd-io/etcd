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
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
)

const (
	defaultEntryTypes string = "Normal,ConfigChange"
)

func main() {
	snapfile := flag.String("start-snap", "", "The base name of snapshot file to start dumping")
	index := flag.Uint64("start-index", 0, "The index to start dumping")
	// Default entry types are Normal and ConfigChange
	entrytype := flag.String("entry-type", defaultEntryTypes, `If set, filters output by entry type. Must be one or more than one of:
ConfigChange, Normal, Request, InternalRaftRequest,
IRRRange, IRRPut, IRRDeleteRange, IRRTxn,
IRRCompaction, IRRLeaseGrant, IRRLeaseRevoke, IRRLeaseCheckpoint`)
	streamdecoder := flag.String("stream-decoder", "", `The name of an executable decoding tool, the executable must process
hex encoded lines of binary input (from etcd-dump-logs)
and output a hex encoded line of binary for each input line`)

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
			ss := snap.New(zap.NewExample(), snapDir(dataDir))
			snapshot, err = ss.Load()
		} else {
			snapshot, err = snap.Read(zap.NewExample(), filepath.Join(snapDir(dataDir), *snapfile))
		}

		switch err {
		case nil:
			walsnap.Index, walsnap.Term = snapshot.Metadata.Index, snapshot.Metadata.Term
			nodes := genIDSlice(snapshot.Metadata.ConfState.Voters)
			confstateJson, err := json.Marshal(snapshot.Metadata.ConfState)
			if err != nil {
				confstateJson = []byte(fmt.Sprintf("confstate err: %v", err))
			}
			fmt.Printf("Snapshot:\nterm=%d index=%d nodes=%s confstate=%s\n",
				walsnap.Term, walsnap.Index, nodes, confstateJson)
		case snap.ErrNoSnapshot:
			fmt.Printf("Snapshot:\nempty\n")
		default:
			log.Fatalf("Failed loading snapshot: %v", err)
		}
		fmt.Println("Start dumping log entries from snapshot.")
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

	fmt.Printf("%4s\t%10s\ttype\tdata", "term", "index")
	if *streamdecoder != "" {
		fmt.Printf("\tdecoder_status\tdecoded_data")
	}
	fmt.Println()

	listEntriesType(*entrytype, *streamdecoder, ents)
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

func passIRRLeaseCheckpoint(entry raftpb.Entry) (bool, string) {
	var rr etcdserverpb.InternalRaftRequest
	return entry.Type == raftpb.EntryNormal && rr.Unmarshal(entry.Data) == nil && rr.LeaseCheckpoint != nil, "InternalRaftRequest"
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
		fmt.Printf("%4d\t%10d\tnorm\t%s", entry.Term, entry.Index, rr.String())
	}
}

func printUnknownNormal(entry raftpb.Entry) {
	fmt.Printf("%4d\t%10d\tnorm\t???", entry.Term, entry.Index)
}

func printConfChange(entry raftpb.Entry) {
	fmt.Printf("%4d\t%10d", entry.Term, entry.Index)
	fmt.Printf("\tconf")
	var r raftpb.ConfChange
	if err := r.Unmarshal(entry.Data); err != nil {
		fmt.Printf("\t???")
	} else {
		fmt.Printf("\tmethod=%s id=%s", r.Type, types.ID(r.NodeID))
	}
}

func printRequest(entry raftpb.Entry) {
	var r etcdserverpb.Request
	if err := r.Unmarshal(entry.Data); err == nil {
		fmt.Printf("%4d\t%10d\tnorm", entry.Term, entry.Index)
		switch r.Method {
		case "":
			fmt.Printf("\tnoop")
		case "SYNC":
			fmt.Printf("\tmethod=SYNC time=%q", time.Unix(0, r.Time).UTC())
		case "QGET", "DELETE":
			fmt.Printf("\tmethod=%s path=%s", r.Method, excerpt(r.Path, 64, 64))
		default:
			fmt.Printf("\tmethod=%s path=%s val=%s", r.Method, excerpt(r.Path, 64, 64), excerpt(r.Val, 128, 0))
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
		"IRRLeaseCheckpoint":  {passIRRLeaseCheckpoint},
	}
	filters := make([]EntryFilter, 0)
	for _, et := range entrytypelist {
		if f, ok := validRequest[et]; ok {
			filters = append(filters, f...)
		} else {
			log.Printf(`[%+v] is not a valid entry-type, ignored.
Please set entry-type to one or more of the following:
ConfigChange, Normal, Request, InternalRaftRequest,
IRRRange, IRRPut, IRRDeleteRange, IRRTxn,
IRRCompaction, IRRLeaseGrant, IRRLeaseRevoke, IRRLeaseCheckpoint`, et)
		}
	}

	return filters
}

//  listEntriesType filters and prints entries based on the entry-type flag,
func listEntriesType(entrytype string, streamdecoder string, ents []raftpb.Entry) {
	entryFilters := evaluateEntrytypeFlag(entrytype)
	printerMap := map[string]EntryPrinter{"InternalRaftRequest": printInternalRaftRequest,
		"Request":       printRequest,
		"ConfigChange":  printConfChange,
		"UnknownNormal": printUnknownNormal}
	var stderr bytes.Buffer
	args := strings.Split(streamdecoder, " ")
	cmd := exec.Command(args[0], args[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Panic(err)
	}
	cmd.Stderr = &stderr
	if streamdecoder != "" {
		err = cmd.Start()
		if err != nil {
			log.Panic(err)
		}
	}

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
			if streamdecoder == "" {
				fmt.Println()
				continue
			}

			// if decoder is set, pass the e.Data to stdin and read the stdout from decoder
			io.WriteString(stdin, hex.EncodeToString(e.Data))
			io.WriteString(stdin, "\n")
			outputReader := bufio.NewReader(stdout)
			decoderoutput, currerr := outputReader.ReadString('\n')
			if currerr != nil {
				fmt.Println(currerr)
				return
			}

			decoder_status, decoded_data := parseDecoderOutput(decoderoutput)

			fmt.Printf("\t%s\t%s", decoder_status, decoded_data)
		}
	}

	stdin.Close()
	err = cmd.Wait()
	if streamdecoder != "" {
		if err != nil {
			log.Panic(err)
		}
		if stderr.String() != "" {
			os.Stderr.WriteString("decoder stderr: " + stderr.String())
		}
	}

	fmt.Printf("\nEntry types (%s) count is : %d\n", entrytype, cnt)
}

func parseDecoderOutput(decoderoutput string) (string, string) {
	var decoder_status string
	var decoded_data string
	output := strings.Split(decoderoutput, "|")
	switch len(output) {
	case 1:
		decoder_status = "decoder output format is not right, print output anyway"
		decoded_data = decoderoutput
	case 2:
		decoder_status = output[0]
		decoded_data = output[1]
	default:
		decoder_status = output[0] + "(*WARNING: data might contain deliminator used by etcd-dump-logs)"
		decoded_data = strings.Join(output[1:], "")
	}
	return decoder_status, decoded_data
}
