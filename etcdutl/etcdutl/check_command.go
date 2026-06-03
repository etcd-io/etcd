// Copyright 2024 The etcd Authors
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

package etcdutl

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

// errors used to signal that custom v2 content was detected in
// either the v2store snapshot or the WAL records.
var (
	errV2StoreCustomContent = errors.New("detected custom content in v2store")
	errWALCustomContent     = errors.New("detected custom v2 content in WAL records")
)

// NewCheckCommand returns the cobra command for "check".
func NewCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <subcommand>",
		Short: "commands for checking properties",
	}
	cmd.AddCommand(NewCheckV2StoreCommand())
	return cmd
}

var (
	argCheckV2StoreDataDir string
	argCheckV2StoreWALDir  string
)

// NewCheckV2StoreCommand returns the cobra command for "check v2store".
func NewCheckV2StoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "v2store",
		Short: "Check custom content in v2store",
		Run:   checkV2StoreRunFunc,
	}
	cmd.Flags().StringVar(&argCheckV2StoreDataDir, "data-dir", "", "Required. A data directory not in use by etcd.")
	cmd.Flags().StringVar(&argCheckV2StoreWALDir, "wal-dir", "", "Optional. A dedicated WAL directory.")
	cmd.MarkFlagRequired("data-dir")
	return cmd
}

func checkV2StoreRunFunc(_ *cobra.Command, _ []string) {
	snapDir := datadir.ToSnapDir(argCheckV2StoreDataDir)
	walDir := datadir.ToWalDir(argCheckV2StoreDataDir)
	if argCheckV2StoreWALDir != "" {
		walDir = argCheckV2StoreWALDir
	}

	err := checkV2StoreDataDir(snapDir, walDir)
	v2StoreHasCustom := errors.Is(err, errV2StoreCustomContent)
	walHasCustom := errors.Is(err, errWALCustomContent)

	switch {
	case v2StoreHasCustom && walHasCustom:
		cobrautl.ExitWithError(cobrautl.ExitError,
			errors.New("detected custom v2 content in both v2store and WAL records"))
	case v2StoreHasCustom:
		cobrautl.ExitWithError(cobrautl.ExitError, errV2StoreCustomContent)
	case walHasCustom:
		cobrautl.ExitWithError(cobrautl.ExitError, errWALCustomContent)
	case err != nil:
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	default:
		fmt.Println("No custom content found in both v2store and WAL records.")
	}
}

func checkV2StoreDataDir(snapDir string, walDir string) error {
	var lg = GetLogger()

	walSnaps, err := wal.ValidSnapshotEntries(lg, walDir)
	if err != nil {
		if errors.Is(err, wal.ErrFileNotFound) {
			return nil
		}
		return err
	}

	ss := snap.New(lg, snapDir)
	snapshot, err := ss.LoadNewestAvailable(walSnaps)
	if err != nil && !errors.Is(err, snap.ErrNoSnapshot) {
		return err
	}

	// A nil snapshot means there is no v2 snapshot file on disk.
	// WALs may still contain v2 requests, so we always scan the WALs.
	// When a v2 snapshot exists we only need to scan records after it.
	walStart := walpb.Snapshot{}
	if snapshot != nil {
		walStart.Index = snapshot.Metadata.Index
		walStart.Term = snapshot.Metadata.Term
		walStart.ConfState = &snapshot.Metadata.ConfState
	}

	var errs []error
	if snapshot != nil {
		st := v2store.New(etcdserver.StoreClusterPrefix, etcdserver.StoreKeysPrefix)
		if err := st.Recovery(snapshot.Data); err != nil {
			return fmt.Errorf("failed to recover v2store from snapshot: %w", err)
		}
		if err := assertNoV2StoreContent(st); err != nil {
			errs = append(errs, err)
		}
	}
	if err := checkV2WALRecords(lg, walDir, walStart); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// checkV2WALRecords walks every WAL record starting at walSnap and returns
// errWALCustomContent if any record carries a v2 request that v3.6 would
// refuse to replay.
func checkV2WALRecords(lg *zap.Logger, walDir string, walSnap walpb.Snapshot) error {
	w, err := wal.OpenForRead(lg, walDir, walSnap)
	if err != nil {
		if errors.Is(err, wal.ErrFileNotFound) {
			return nil
		}
		return fmt.Errorf("failed to open WAL: %w", err)
	}
	defer w.Close()

	// When walSnap is zero, ReadAll returns ErrSnapshotNotFound
	_, _, ents, err := w.ReadAll()
	if err != nil && !errors.Is(err, wal.ErrSnapshotNotFound) {
		return fmt.Errorf("failed to read WAL: %w", err)
	}

	for i := range ents {
		if err := assertNoCustomV2Request(&ents[i]); err != nil {
			return err
		}
	}
	return nil
}

// assertNoCustomV2Request returns errWALCustomContent if the entry carries a
// v2 request that v3.6 would panic on replay.
func assertNoCustomV2Request(e *raftpb.Entry) error {
	if e.Type != raftpb.EntryNormal || len(e.Data) == 0 {
		return nil
	}

	var raftReq pb.InternalRaftRequest
	var v2Req *pb.Request
	// v2 request is wrapped inside pb.InternalRaftRequest
	if pbutil.MaybeUnmarshal(&raftReq, e.Data) {
		v2Req = raftReq.V2
	} else {
		// legacy v2 request directly stored in Entry.Data
		v2Req = &pb.Request{}
		if err := v2Req.Unmarshal(e.Data); err != nil {
			return fmt.Errorf("failed to unmarshal WAL entry at index %d: %w", e.Index, err)
		}
	}
	if v2Req == nil {
		return nil
	}

	// must satisfy v3.6's v2ToV3Request constraints, otherwise the upgrading panics on replay
	if v2Req.Method == http.MethodPut &&
		(etcdserver.StoreMemberAttributeRegexp.MatchString(v2Req.Path) ||
			v2Req.Path == membership.StoreClusterVersionKey()) {
		return nil
	}

	return errWALCustomContent
}

func assertNoV2StoreContent(st v2store.Store) error {
	metaOnly, err := membership.IsMetaStoreOnly(st)
	if err != nil {
		return err
	}
	if metaOnly {
		return nil
	}
	return errV2StoreCustomContent
}
