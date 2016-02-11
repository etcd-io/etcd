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
	"io"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
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
		snapshotToStdout(mustClient(cmd))
	case len(args) == 1:
		snapshotToFile(mustClient(cmd), args[0])
	default:
		err := fmt.Errorf("snapshot takes at most one argument")
		ExitWithError(ExitBadArgs, err)
	}
}

// snapshotToStdout streams a snapshot over stdout
func snapshotToStdout(c *clientv3.Client) {
	// must explicitly fetch first revision since no retry on stdout
	wapi := clientv3.NewWatcher(c)
	defer wapi.Close()
	wr := <-wapi.WatchPrefix(context.TODO(), "", 1)
	if len(wr.Events) > 0 {
		wr.CompactRevision = 1
	}
	if rev := snapshot(os.Stdout, c, wr.CompactRevision); rev != 0 {
		err := fmt.Errorf("snapshot interrupted by compaction %v", rev)
		ExitWithError(ExitInterrupted, err)
	}
}

// snapshotToFile atomically writes a snapshot to a file
func snapshotToFile(c *clientv3.Client, path string) {
	partpath := path + ".part"
	f, err := os.Create(partpath)
	defer f.Close()
	if err != nil {
		exiterr := fmt.Errorf("could not open %s (%v)", partpath, err)
		ExitWithError(ExitBadArgs, exiterr)
	}
	rev := int64(1)
	for rev != 0 {
		f.Seek(0, 0)
		f.Truncate(0)
		rev = snapshot(f, c, rev)
	}
	f.Sync()
	if err := os.Rename(partpath, path); err != nil {
		exiterr := fmt.Errorf("could not rename %s to %s (%v)", partpath, path, err)
		ExitWithError(ExitIO, exiterr)
	}
}

// snapshot reads all of a watcher; returns compaction revision if incomplete
// TODO: stabilize snapshot format
func snapshot(w io.Writer, c *clientv3.Client, rev int64) int64 {
	wapi := clientv3.NewWatcher(c)
	defer wapi.Close()

	// get all events since revision (or get non-compacted revision, if
	// rev is too far behind)
	wch := wapi.WatchPrefix(context.TODO(), "", rev)
	for wr := range wch {
		if len(wr.Events) == 0 {
			return wr.CompactRevision
		}
		for _, ev := range wr.Events {
			fmt.Fprintln(w, ev)
		}
		rev := wr.Events[len(wr.Events)-1].Kv.ModRevision
		if rev >= wr.Header.Revision {
			break
		}
	}

	// get base state at rev
	kapi := clientv3.NewKV(c)
	key := "\x00"
	for {
		kvs, err := kapi.Get(
			context.TODO(),
			key,
			clientv3.WithFromKey(),
			clientv3.WithRev(rev+1),
			clientv3.WithLimit(1000))
		if err == v3rpc.ErrCompacted {
			// will get correct compact revision on retry
			return rev + 1
		} else if err != nil {
			// failed for some unknown reason, retry on same revision
			return rev
		}
		for _, kv := range kvs.Kvs {
			fmt.Fprintln(w, kv)
		}
		if !kvs.More {
			break
		}
		// move to next key
		key = string(append(kvs.Kvs[len(kvs.Kvs)-1].Key, 0))
	}

	return 0
}
