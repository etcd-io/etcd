// Copyright 2016 CoreOS, Inc.
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

package command

import (
	"fmt"
	"os"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/mirror"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/coreos/etcd/storage"
	"github.com/coreos/etcd/storage/backend"
	spb "github.com/coreos/etcd/storage/storagepb"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// NewSnapshotCommand returns the cobra command for "snapshot".
func NewSnapshotCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot [filename]",
		Short: "Snapshot streams a point-in-time snapshot of the store",
		Run:   snapshotCommandFunc,
	}
}

// snapshotCommandFunc watches for the length of the entire store and records
// to a file.
func snapshotCommandFunc(cmd *cobra.Command, args []string) {
	switch {
	case len(args) == 0:
		snapshotToStdout(mustClientFromCmd(cmd))
	case len(args) == 1:
		snapshotToFile(mustClientFromCmd(cmd), args[0])
	default:
		err := fmt.Errorf("snapshot takes at most one argument")
		ExitWithError(ExitBadArgs, err)
	}
}

// snapshotToStdout streams a snapshot over stdout
func snapshotToStdout(c *clientv3.Client) {
	// must explicitly fetch first revision since no retry on stdout
	wr := <-c.Watch(context.TODO(), "", clientv3.WithPrefix(), clientv3.WithRev(1))
	if wr.Err() == nil {
		wr.CompactRevision = 1
	}

	wch := make(chan []*spb.Event)
	donec := make(chan struct{})
	go func() {
		for evs := range wch {
			display.Watch(clientv3.WatchResponse{Events: evs})
		}
		os.Stdout.Sync()
		close(donec)
	}()
	if rev := snapshot(wch, c, wr.CompactRevision+1); rev != 0 {
		err := fmt.Errorf("snapshot interrupted by compaction %v", rev)
		ExitWithError(ExitInterrupted, err)
	}
	close(wch)
	<-donec
}

// snapshotToFile atomically writes a snapshot to a file
func snapshotToFile(c *clientv3.Client, path string) {
	var b backend.Backend
	partpath := path + ".part"
	rev := int64(1)
	for rev != 0 {
		if b != nil {
			os.Remove(partpath)
			b.Close()
		}
		b = backend.NewDefaultBackend(partpath)

		wch := make(chan []*spb.Event)
		donec := make(chan struct{})
		go func() {
			if err := storage.WriteBackend(b, wch); err != nil {
				ExitWithError(ExitIO, err)
			}
			close(donec)
		}()
		rev = snapshot(wch, c, rev)
		close(wch)
		<-donec
	}
	b.Close()

	if err := os.Rename(partpath, path); err != nil {
		exiterr := fmt.Errorf("could not rename %s to %s (%v)", partpath, path, err)
		ExitWithError(ExitIO, exiterr)
	}
}

// snapshot reads all of a watcher; returns compaction revision if incomplete
func snapshot(wch chan<- []*spb.Event, c *clientv3.Client, rev int64) int64 {
	s := mirror.NewSyncer(c, "", rev)

	rc, errc := s.SyncBase(context.TODO())
	for r := range rc {
		evs := make([]*spb.Event, len(r.Kvs))
		for i, kv := range r.Kvs {
			evs[i] = &spb.Event{Type: spb.PUT, Kv: kv}
		}
		wch <- evs
	}

	err := <-errc
	if err != nil {
		if err == rpctypes.ErrCompacted {
			// will get correct compact revision on retry
			return rev + 1
		}
		// failed for some unknown reason, retry on same revision
		return rev
	}

	wc := s.SyncUpdates(context.TODO())

	for wr := range wc {
		if wr.Err() != nil {
			return wr.CompactRevision
		}
		rev := wr.Events[len(wr.Events)-1].Kv.ModRevision
		if rev >= wr.Header.Revision {
			break
		}
		wch <- wr.Events
	}

	return 0
}
