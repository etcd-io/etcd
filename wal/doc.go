/*
   Copyright 2014 CoreOS, Inc.

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

/*
Package wal provides an implementation of a write ahead log that is used by
etcd.

A WAL is created at a particular directory and is made up of a number of
discrete WAL files. Inside of each file the raft state and entries are appended
to it with the Save method:

	metadata := []byte{}
	w, err := wal.Create("/var/lib/etcd", metadata)
	...
	err := w.Save(s, ents)

When a user has finished using a WAL it must be closed:

	w.Close()

WAL files are placed inside of the directory in the following format:
$seq-$index.wal

The first WAL file to be created will be 0000000000000000-0000000000000000.wal
indicating an initial sequence of 0 and an initial raft index of 0. The first
entry written to WAL MUST have raft index 0.

Periodically a user will want to "cut" the WAL and place new entries into a new
file. This will increment an internal sequence number and cause a new file to
be created. If the last raft index saved was 0x20 and this is the first time
Cut has been called on this WAL then the sequence will increment from 0x0 to
0x1. The new file will be: 0000000000000001-0000000000000021.wal. If a second
Cut issues 0x10 entries with incremental index later then the file will be called:
0000000000000002-0000000000000031.wal.

At a later time a WAL can be opened at a particular raft index:

	w, err := wal.OpenAtIndex("/var/lib/etcd", 0)
	...

The raft index must have been written to the WAL. When opening without a
snapshot the raft index should always be 0. When opening with a snapshot
the raft index should be the index of the last entry covered by the snapshot.

Additional items cannot be Saved to this WAL until all of the items from 0 to
the end of the WAL are read first:

	id, state, ents, err := w.ReadAll()

This will give you the raft node id, the last raft.State and the slice of
raft.Entry items in the log.

*/
package wal
