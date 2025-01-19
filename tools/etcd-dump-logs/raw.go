// Copyright 2022 The etcd Authors
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
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3/raftpb"
)

func readRaw(fromIndex *uint64, waldir string, out io.Writer) {
	var walReaders []fileutil.FileReader
	dirEntry, err := os.ReadDir(waldir)
	if err != nil {
		log.Fatalf("Error: Failed to read directory '%s' error:%v", waldir, err)
	}
	for _, e := range dirEntry {
		finfo, err := e.Info()
		if err != nil {
			log.Fatalf("Error: failed to get fileInfo of file: %s, error: %v", e.Name(), err)
		}
		if filepath.Ext(finfo.Name()) != ".wal" {
			log.Printf("Warning: Ignoring not .wal file: %s", finfo.Name())
			continue
		}
		f, err := os.Open(filepath.Join(waldir, finfo.Name()))
		if err != nil {
			log.Printf("Error: Failed to read file: %s . error:%v", finfo.Name(), err)
		}
		walReaders = append(walReaders, fileutil.NewFileReader(f))
	}
	decoder := wal.NewDecoderAdvanced(true, walReaders...)
	// The variable is used to not pollute log with multiple continuous crc errors.
	crcDesync := false
	for {
		rec := walpb.Record{}
		err := decoder.Decode(&rec)
		if err == nil || errors.Is(err, walpb.ErrCRCMismatch) {
			if err != nil && !crcDesync {
				log.Printf("Error: Reading entry failed with CRC error: %c", err)
				crcDesync = true
			}
			printRec(&rec, fromIndex, out)
			if rec.Type == wal.CrcType {
				decoder.UpdateCRC(rec.Crc)
				crcDesync = false
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			fmt.Fprintf(out, "EOF: All entries were processed.\n")
			break
		} else if errors.Is(err, io.ErrUnexpectedEOF) {
			fmt.Fprintf(out, "ErrUnexpectedEOF: The last record might be corrupted, error: %v.\n", err)
			break
		}
		log.Printf("Error: Reading failed: %v", err)
		break
	}
}

func printRec(rec *walpb.Record, fromIndex *uint64, out io.Writer) {
	switch rec.Type {
	case wal.MetadataType:
		var metadata etcdserverpb.Metadata
		pbutil.MustUnmarshal(&metadata, rec.Data)
		fmt.Fprintf(out, "Metadata: %s\n", metadata.String())
	case wal.CrcType:
		fmt.Fprintf(out, "CRC: %d\n", rec.Crc)
	case wal.EntryType:
		e := wal.MustUnmarshalEntry(rec.Data)
		if fromIndex == nil || e.Index >= *fromIndex {
			fmt.Fprintf(out, "Entry: %s\n", e.String())
		}
	case wal.SnapshotType:
		var snap walpb.Snapshot
		pbutil.MustUnmarshal(&snap, rec.Data)
		if fromIndex == nil || snap.Index >= *fromIndex {
			fmt.Fprintf(out, "Snapshot: %s\n", snap.String())
		}
	case wal.StateType:
		var state raftpb.HardState
		pbutil.MustUnmarshal(&state, rec.Data)
		if fromIndex == nil || state.Commit >= *fromIndex {
			fmt.Fprintf(out, "HardState: %s\n", state.String())
		}
	default:
		log.Printf("Unexpected WAL log type: %d", rec.Type)
	}
}
