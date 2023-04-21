# Etcd persistent storage files

Last updated : etcd-v3.5 (2023-03-15)


## Purpose

The document explains the etcd persistent storage format: naming, content and tools that allow developers to inspect them. Going forward the document should be extended with changes to the storage model. This document is targeted at etcd developers to help with their data recovery needs.


## Prerequisites

The following articles provide helpful background information for this document: 

* Etcd data model overview: https://etcd.io/docs/v3.4/learning/data_model
* Raft overview: http://web.stanford.edu/~ouster/cgi-bin/papers/Raft-atc14.pdf (especially "5.3 Log replication" section).


## Overview

### Long leaving files

<table style="border: 1px solid black;">
  <tr style="border: 1px solid black;">
   <td><strong>File name</strong></td>
   <td><strong>High level purpose</strong></td>
  </tr>
 
  <tr style="outline: thin solid">
   <td>./member/snap/db</td>
   <td><strong>bbolt <a href="https://en.wikipedia.org/wiki/B%2B_tree">b+tree</a></strong> that stores all the applied data, membership authorization information & metadata. It’s aware of what's the last applied WAL log index (<a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/etcdserver/cindex/cindex.go#L92">"consistent_index"</a>). 
   </td>
  </tr>

  <tr style="outline: thin solid">
   <td><pre>
./member/snap/0000000000000002-0000000000049425.snap
./member/snap/0000000000000002-0000000000061ace.snap
   </pre></td>
   <td>Periodic <strong>snapshots of legacy v2 store</strong>, containing:
<ul>
  <li>basic membership information
  <li>etcd-version
</ul>
As of etcd v3, the content is redundant to the content of /snap/db files.

Periodically (<a href="https://github.com/etcd-io/etcd/blob/a4570a60e771402360755beb7d662bdbca1f87f2/server/etcdserver/server.go#L87">30s</a>) these files <a href="etcdserver/server.go:774">are purged</a>, and the last `--max-snapshots=5` are preserved.
   </td>
  </tr>

 <tr style="outline: thin solid">
   <td><pre>/member/snap/000000000007a178.snap.db</pre></td>
   <td>A complete <strong>bbolt snapshot downloaded</strong> from the etcd leader if the replica was lagging too much. 
<p>
Has the same type of content as (./member/snap/db) file. 
<p>
The file is used in 2 scenarios: 
<ul>
  <li>In response to the leader's request to recover from the snapshot.
  <li><a href="https://github.com/etcd-io/etcd/blob/a4570a60e771402360755beb7d662bdbca1f87f2/server/etcdserver/server.go#L444">During the server startup</a>, when the last snapshot (.snap.db file) is found and detected to be having a newer index than the  consistent_index in the current `snap.db` file.</li>
</ul>

Note: Periodic snapshots generated on each replica are only emitted in the form of *.snap file (not snap.db file). So there is no guarantee the most recent snapshot (in WAL log) has the *.snap.db file. But in such a case the backend (snap/db) is expected to be newer than the snapshot.

The file<a href="https://github.com/etcd-io/etcd/blob/a4570a60e771402360755beb7d662bdbca1f87f2/server/etcdserver/server.go#L1236"> is not being deleted when the recovery is over</a> (so whole content is populated to ./member/snap/db file) Periodically (<a href="https://github.com/etcd-io/etcd/blob/a4570a60e771402360755beb7d662bdbca1f87f2/server/etcdserver/server.go#L87">30s</a>) the files <a href="https://github.com/etcd-io/etcd/blob/a4570a60e771402360755beb7d662bdbca1f87f2/server/etcdserver/server.go#L774">are</a> purged. 
Here also `--max-snapshots=5` are preserved. As these files can be O(GBs) this might create a risk of disk space exhaustion.
</li>
</ul>
   </td>
  </tr>

  <tr style="outline: thin solid">
   <td><pre>
./member/wal/000000000000000f-00000000000b38c7.wal
./member/wal/000000000000000e-00000000000a7fe3.wal
./member/wal/000000000000000d-000000000009c70c.wal
   </pre></td>
   <td><strong>Raft’s Write Ahead Logs</strong>, containing recent transactions accepted by Raft, periodic snapshots or CRC records.

Recent `--max-wals=5` files are being preserved. Each of these files is `~64*10^6` bytes. The file is cut when it exceeds this hardcoded size, so the files might slightly exceed that size (so the preallocated `0.tmp` does not offer full disk-exceeded protection).
 
If the snapshots are too infrequent, there can be more than `--max-wals=5`, as file-system level locks are protecting the files preventing them from being deleted too early. 
   </td>
  </tr>
  
  <tr style="outline: thin solid">
   <td><pre>./member/wal/0.tmp (or .../1.tmp)</pre>
   </td>
   <td><strong>Preallocated space</strong> for the next write ahead log file.

Used to avoid Raft being stuck by a lack of WAL logs capacity without the possibility to raise an alarm.
   </td>
  </tr>
</table>


### Temporary files
During etcd internal processing, it is possible that several short living files might be encountered:


<table style="border: 1px solid black;">
  <tr style="outline: thin solid">
   <td><strong>File</strong>
   </td>
   <td><strong>High level purpose</strong>
   </td>
  </tr>
  <tr  style="outline: thin solid">
   <td>./member/snap/ \
0000000000000002-000000000007a178.snap.broken \

   </td>
   <td>Snapshot files are renamed as ‘broken’ when they cannot be loaded: 
<p>
TODO: etcdserver/api/snap/snapshotter.go:148
<p>
The attempt to load the newest file happens when etcd is being started: 
<p>
TODO: etcdserver/server.go:428 \
Or during backup/migrate commands of etcdctl.
   </td>
  </tr>
  <tr  style="outline: thin solid">
   <td>/member/snap/tmp071677638 (random suffix)
   </td>
   <td><a href="https://github.com/etcd-io/etcd/blob/ae9734ed278b7a1a7dfc82e800471ebbf9fce56f/etcdserver/api/snap/db.go#L39">Temporary</a> (bbolt) file created on replicas in response to the msgSnap leaders request, so to the demand from the leader to recover storage from the given snapshot.
<p>
After successful (complete) retrieval of content the file is <a href="https://github.com/etcd-io/etcd/blob/ae9734ed278b7a1a7dfc82e800471ebbf9fce56f/etcdserver/api/snap/db.go#L55">renamed</a> to:  \
/member/snap/<code>[SNAPSHOT-INDEX].snap.db \
 \
</code>In case of a server dying / being killed in the middle of the files download, the files remain on disk and are never automatically cleaned.They can be substantial in size (GBs).  \
See <a href="https://github.com/etcd-io/etcd/issues/12837">etcd/issues/12837</a>. <strong>Fixed in etcd 3.5.</strong>
   </td>
  </tr>
  <tr  style="outline: thin solid">
   <td>/member/snap/db.tmp.071677638 (random suffix)
   </td>
   <td>A temporary file that contains a copy of the backend content (/member/snap/db), during the <a href="https://github.com/etcd-io/etcd/blob/a4570a60e771402360755beb7d662bdbca1f87f2/server/mvcc/backend/backend.go#L373">process of defragmentation</a>. After the successful process the file is renamed to /member/snap/db, replacing the original backend. 
<p>
 On etcd server startup these files get<a href="https://github.com/etcd-io/etcd/blob/a4570a60e771402360755beb7d662bdbca1f87f2/server/etcdserver/api/snap/snapshotter.go#L217"> pruned</a>.
   </td>
  </tr>
</table>



## Bbolt b+tree: **member/snap/db **

This file contains the main etcd content, applied to a specific point of the Raft log (see [consistent_index](https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/etcdserver/cindex/cindex.go#L92)). 


### Physical organization 

The better bolt storage is physically organized as a [b+tree](https://en.wikipedia.org/wiki/B%2B_tree). The physical pages of b-tree are never modified in-place[^1]. Instead, the content is copied to a new page (reclaimed from the freepages list) and the old page is added to the free-pages list as soon as there is no open transaction that might access it. Thanks to this process, an open RO transaction sees a consistent historical state of the storage. The RW transaction is exclusive and blocking all other RW transactions.  \
Big values are stored on multiple continuous pages. The process of page reclamation combined with a need to allocate contiguous areas of pages of different sizes might lead to growing fragmentation of the bbolt storage. 

The bbolt file never shrinks on its own. Only in the defragmentation process, the file can be rewritten to a new one that has some buffer of free pages on its end and has truncated size. 


### Logical organization

The bbolt storage is divided into buckets. In each bucket there are stored keys (byte[]->value byte[] pairs), in lexicographical order.  The list below represents buckets used by etcd (as of version 3.5) and the keys in use. 


<table>
  <tr>
   <td><strong>bucket</strong>
   </td>
   <td><strong>key</strong>
   </td>
   <td><strong>Exemplar value</strong>
   </td>
   <td><strong>description</strong>
   </td>
  </tr>
  <tr>
   <td>alarm
   </td>
   <td><code><a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/api/etcdserverpb/rpc.proto#L976">rpcpb.Alarm</a>:</code>
<code>{MemberID, Alarm: NONE|NOSPACE|CORRUPT}</code>
   </td>
   <td>nil
   </td>
   <td>Indicates problems have been diagnosed in one of the members.
   </td>
  </tr>
  <tr>
   <td>auth
   </td>
   <td>"authRevision"
   </td>
   <td>""(empty) or \
<code>BigEndian.PutUint64</code>
   </td>
   <td>Any change of Roles or Users increments this field on transaction commit.
<p>
The value is used only for <a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/etcdserver/v3_server.go#L461">optimistic locking during</a> the authorization process.
   </td>
  </tr>
  <tr>
   <td>authRoles
   </td>
   <td>[roleName] as string
   </td>
   <td><code><a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/api/authpb/auth.proto#L38">authpb.Role</a> </code>marshalled
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td>authUsers
   </td>
   <td>[userName] as string
   </td>
   <td><code><a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/api/authpb/auth.proto#L17">authpb.User</a> </code>marshalled
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td rowspan="2" >cluster
   </td>
   <td>"clusterVersion"
   </td>
   <td>"3.5.0" (string)
   </td>
   <td><a href="https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/etcdserver/server.go#L2314">minor</a> version of consensus-agreed common storage version. 
   </td>
  </tr>
  <tr>
   <td>“downgrade”
   </td>
   <td>Json: { \
“target-version”: "3.4.0" \
“enabled”: true/false
<p>
}
   </td>
   <td>Persists intent configured by the most recent: <code>Downgrade RPC </code>request.
<p>
Since v3.5
   </td>
  </tr>
  <tr>
   <td><strong>key</strong>
   </td>
   <td>[revisionId]
<p>
encoded using
<p>
<a href="https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/mvcc/revision.go#L56">bytesToRev</a>{main,sub} 
<p>
The key-value deletes are marshalled with ‘t’ at the end (as a “Thumbstone”)
   </td>
   <td><a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/api/mvccpb/kv.proto#L12">mvccpb.KeyValue</a> marshalled proto (<code>key, create_rev, mod_rev, version, value, lease id</code>)
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td>lease
   </td>
   <td>
   </td>
   <td><code><a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/lease/leasepb/lease.proto#L13">leasepb.Lease</a> </code> marshalled proto (ID, TTL, RemainingTTL)
   </td>
   <td>Note: LeaseCheckpoint is extending only RemainingTTL. Just TTL is from the original Grant.  
<p>
<span style="text-decoration:underline;">Note2: We persist TTLs in seconds (from the undefined ‘now’). Crash-looping server does not release leases !!!</span>
   </td>
  </tr>
  <tr>
   <td>members
   </td>
   <td>[memberId] in hex as string:  \
 \
"8e9e05c52164694d"
   </td>
   <td><em>json as string serialized <a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/etcdserver/api/membership/member.go#L43">Member</a> structure:</em> \
{"id":10276657743932975437, \
"peerURLs":[ \
  "<a href="http://localhost:2380">http://localhost:2380</a>"], \
"name":"default", \
"clientURLs": \
   ["http://localhost:2379"]}
   </td>
   <td>Agreed cluster membership information. 
   </td>
  </tr>
  <tr>
   <td>members \
_removed
   </td>
   <td>[memberId] in hex as string:  \
 \
"8e9e05c52164694d"
   </td>
   <td><code>[]byte("removed")</code>
   </td>
   <td>Ids of all removed members. \
 \
Used to validate that a removed member is never added again under the same id. 
<p>
<a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/etcdserver/api/membership/cluster.go#L251">The field is currently (3.4) read from store V2 and never from V3.</a> See <a href="https://github.com/etcd-io/etcd/pull/12820">https://github.com/etcd-io/etcd/pull/12820</a> 
   </td>
  </tr>
  <tr>
   <td rowspan="3" ><strong>meta</strong>
   </td>
   <td><a href="https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/etcdserver/cindex/cindex.go#L92">"consistent_index"</a>
   </td>
   <td>uint64 bytes (BigEndian)
   </td>
   <td>Represents the offset of the last applied WAL entry to the bolt DB storage.
   </td>
  </tr>
  <tr>
   <td><a href="https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/mvcc/kvstore.go#L41">"scheduledCompactRev"</a>
   </td>
   <td><a href="https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/mvcc/revision.go#L56">bytesToRev</a>{main,sub} encoded. (16 bytes)
   </td>
   <td>Used to reinitialize compaction if a crash happened after a compaction request. 
   </td>
  </tr>
  <tr>
   <td><a href="https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/mvcc/kvstore.go#L42">"finishedCompactRev"</a>
   </td>
   <td><a href="https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/mvcc/revision.go#L56">bytesToRev</a>{main,sub} encoded. (16 bytes)
   </td>
   <td>Revision at which store was recently successfully compacted (<a href="https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/mvcc/kvstore_compaction.go#L54">https://github.com/etcd-io/etcd/blob/ae7862e8bc8007eb396099db4e0e04ac026c8df5/server/mvcc/kvstore_compaction.go#L54</a>)
   </td>
  </tr>
  <tr>
   <td>
   </td>
   <td>“confState” 
   </td>
   <td>
   </td>
   <td><a href="https://github.com/etcd-io/etcd/pull/12962">Since etcd 3.5</a>
   </td>
  </tr>
  <tr>
   <td>
   </td>
   <td>“term”
   </td>
   <td>
   </td>
   <td><a href="https://github.com/etcd-io/etcd/pull/12964">Since etcd 3.5</a>
   </td>
  </tr>
  <tr>
   <td>
   </td>
   <td>“storage-version”
   </td>
   <td>
   </td>
   <td>
   </td>
  </tr>
</table>



### Tools


#### BBolt

Bbolt has a command line tool that enables inspecting the file content. 

Examples of use: 


##### List all buckets in given bbolt file:

% gobin --run go.etcd.io/bbolt/cmd/bbolt buckets ./default.etcd/member/snap/db


##### Read a particular key/value pair: 

% gobin --run go.etcd.io/bbolt/cmd/bbolt get ./default.etcd/member/snap/db cluster clusterVersion


#### etcd-dump-db

etcd-dump-db can be used to list content of v3 etcd backend (bbolt).

% go run go.etcd.io/etcd/v3/tools/etcd-dump-db  list-bucket default.etcd

alarm

auth

...

See more examples in:  [https://github.com/etcd-io/etcd/tree/master/tools/etcd-dump-db](https://github.com/etcd-io/etcd/tree/master/tools/etcd-dump-db)


## 


## WAL: Write ahead log

Write ahead log is a Raft persistent storage that is used to store proposals. First the leader stores the proposal in its log and then (concurrently) replicates it using Raft protocol to followers. Each follower persists the proposal in its WAL before confirming back replication to the leader.

 \
The WAL log used in etcd differs from canonical Raft model 2-fold:



* It does persist not only indexed entries, but also Raft snapshots (lightweight) & hard-state. So the entire Raft state of the member can be recovered from the WAL log alone.
* It is append-only. Entries are not overridden in place, but an entry appended later in the file (with the same index) is superseding the previous one. 


### File names

The WAL log files are named using following pattern: 


```
"%016x-%016x.wal", seq, index
```


Example: ./member/wal/0000000000000010-00000000000bf1e6.wal

So the file names contains hex-encoded: 



* Sequential number of the WAL log file
* Index of the first entry or snapshot in the file.  \
In particular the first file “0000000000000000-0000000000000000.wal” has the initial snapshot record with index=0. \



### Physical content

The WAL log file contains a sequence of "[Frames](https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/wal/encoder.go#L62)". Each frame contains:



1. [LittleEndian](https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/wal/encoder.go#L120)[^2] encoded uint64 that contains the length of the marshalled [walpb.Record](https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/wal/walpb/record.proto#L11) (3). 


2. Padding: Some number of 0 bytes, such that whole frame has aligned (mod 8) size
3. Marshalled [walpb.Record](https://github.com/etcd-io/etcd/blob/a1ff0d5373335665b3e5f4cb22a538ac63757cb6/server/wal/walpb/record.proto#L11) data:
    1. [type](wal/wal.go:39) - int encoded enum driving interpretation of the data-field below
    2. data - depending on type, usually marshalled proto 
    3. crc - RC-32 checksum of all “data” fields combined (no type) in all the log records on this particular replica since WAL log creation. Please note that CRC takes in consideration ALL records (even if they didn’t get committedcomitted by Raft). 

The files are “cut” (new file is started) when the current file is exceeding 64*10^6 bytes. 


### Logical content

Write ahead log files in the logical layer contains:



* `Raftpb.Entry: `recent proposals replicated by Raft leader. Some of these proposals are considered ‘committed’ and the others are subject to be logically overridden. 
* `Raftpb.HardState(term,commit,vote): `periodic (very frequent) information about the index of a log entry that is ‘committed’ (replicated to the majority of servers), so  guaranteed to be not changed/overridden and that can be applied to the backends (v2, v3). It also contains a “term” (indicator whether there were any election related changes) and a vote - a member the current replica voted for in the current term. 
* `walpb.Snapshot(term, index): `periodic snapshots of Raft state (no DB content, just snapshot log index and Raft term)
    * V2 store content is stored in a separate *.store files.
    * V3 store content is maintained in the bbolt file, and it’s becoming an implicit snapshot as soon as entries are applied there.
* crc32 checksum record (at the beginning of each file), used to resume CRC checking for the remainder of the file. 
* `etcdserverpb.Metadata(node_id, cluster_id)` - identifying the cluster & replica the log represents. 

Each WAL-log file is build from (in order):



1. CRC-32 frame (running crc from all previous files, 0 for the first file).
2. Metadata frame (cluster & replica IDs)
3. For the initial WAL file only: 

        Empty Snapshot frame (Index:0, Term: 0) \
The purpose of this frame is to hold an invariant that all entries are ‘preceded’ by a snapshot. 


    For not initial (2nd+) WAL file:


	HardState frame.



4. Mix of entry, hard-state & snapshot records

The WAL log can contain multiple entries for the same index. Such a situation can happen in cases described in figure 7. of the [Raft paper](http://web.stanford.edu/~ouster/cgi-bin/papers/raft-atc14.pdf). The etcd WAL log is appended only, so the entries are getting overridden, by appending a new entry with the same index. 

In particular during the WAL reading, [the logic is overriding old entries with newer entries](https://github.com/etcd-io/etcd/blob/release-3.4/wal/wal.go#L448-L462). Thus only the last version of entries with entry.index &lt;= HardState.commit can be considered as final. Entries with index > HardState.commit are subject to change.

The “terms” in the WAL log are expected to be monotonic. 

The “indexes” in the WAL log are expected to:



1. start from some snapshot
2. be sequentially growing after that snapshot as long as they stay in the same ‘term’
3. if the term changes, the index can decrease, but to a new value that is higher than the latest HardState.commit.
4. a new snapshot might happen with any index >= HardState.commit, that opens a new sequence for indexes.



<p id="gdcalert1" ><span style="color: red; font-weight: bold">>>>>>  gd2md-html alert: inline drawings not supported directly from Docs. You may want to copy the inline drawing to a standalone drawing and export by reference. See <a href="https://github.com/evbacher/gd2md-html/wiki/Google-Drawings-by-reference">Google Drawings by reference</a> for details. The img URL below is a placeholder. </span><br>(<a href="#">Back to top</a>)(<a href="#gdcalert2">Next alert</a>)<br><span style="color: red; font-weight: bold">>>>>> </span></p>


![drawing](https://docs.google.com/drawings/d/12345/export/png)



### Tools


#### etcd-dump-logs

etcd WAL logs can be read using [etcd-dump-logs](https://github.com/etcd-io/etcd/tree/master/tools/etcd-dump-logs) tool:

% go install go.etcd.io/etcd/v3/tools/etcd-dump-logs@latest

% go run go.etcd.io/etcd/v3/tools/etcd-dump-logs --start-index=0 aname.etcd

Be aware that: 



* The tool shows only Entries, and not all the WAL records (Snapshots, HardStates) that are in the WAL log files. 
* The tool automatically applies ‘overrides’ on the entries. If an entry got overridden (by a fresher entry under the same index), the tool will print only the final value.  
* The tool also prints uncommitted entries (from the tail of the LOG), without information about HardState.commitIndex, so it’s not known whether entries are final or not. 


## Snapshots of (Store V2): **member/snap/{term}-{index}.snap **


### File names: 

**member/snap/{term}-{index}.snap **

The filenames are generated [here](https://github.com/etcd-io/etcd/blob/ad5b30297a43daeb5ce7311fa606ce4c1f16618f/server/etcdserver/api/snap/snapshotter.go#L78) `("%016x-%016x.snap") `and are using 2 hex-encoded compounds:



* term -> Raft term (period between elections) at the time snapshot is emitted
* index -> of last applied proposal at the time snapshot is emitted


### Creation

The *.snap files are created by [Snapshotter.SaveSnap](https://github.com/etcd-io/etcd/blob/ad5b30297a43daeb5ce7311fa606ce4c1f16618f/server/etcdserver/api/snap/snapshotter.go#L68) method. 

There are 2 triggers controlling creation of these files: 



* A new file is created every (approximately) --snapshotCount=(by default 100'000)  applied proposals. It’s an approximation as we might receive proposals in batches and we consider snapshotting only at the end of batch, finally the snapshotting process is asynchronously scheduled.   \
The flag name (--snapshotCount) is pretty misleading as it drives [differences in index value between ](https://github.com/etcd-io/etcd/blob/ad5b30297a43daeb5ce7311fa606ce4c1f16618f/server/etcdserver/server.go#L1266)last snapshot index and last applied proposal index. \

* Raft requests the replica to restore from the snapshot. As a replica is receiving the snapshot over wire (msgSnap) message, it also checkpoints (lightweight) it into WAL log. This guarantees that in the WAL logs tail there is always a valid snapshot followed by entries. So it suppresses potential lack of continuity in the WAL logs.

Currently the files are roughly[^3] associated 1-1 with WAL logs Snapshot entries. With store v2 decommissioning we expect the files to stop being written at all (opt-in: 3.5.x,  mandatory 3.6.x). 


### Content

The file contains marshalled [snapdb.snapshot proto](https://github.com/etcd-io/etcd/blob/ad5b30297a43daeb5ce7311fa606ce4c1f16618f/server/etcdserver/api/snap/snappb/snap.proto#L11) `(uint32 crc, bytes data)`,

that in the 'data' field holds [Raftpb.Snapshot](https://github.com/etcd-io/etcd/blob/ad5b30297a43daeb5ce7311fa606ce4c1f16618f/raft/raftpb/raft.proto#L31):

(bytes data, SnapshotMetadata{index, term, [conf](https://github.com/etcd-io/etcd/blob/ad5b30297a43daeb5ce7311fa606ce4c1f16618f/raft/raftpb/raft.proto#L99)} metadata), 

Finally the nested data holds a JSON serialized 

<p id="gdcalert2" ><span style="color: red; font-weight: bold">>>>>>  gd2md-html alert: undefined internal link (link text: "store v2 content"). Did you generate a TOC with blue links? </span><br>(<a href="#">Back to top</a>)(<a href="#gdcalert3">Next alert</a>)<br><span style="color: red; font-weight: bold">>>>>> </span></p>

[store v2 content](#heading=h.j3kt3frgwhrh).

In particular there is: 



* Term
* Index
* Membership data: 
    * <code>/0/members/8e9e05c52164694d/attributes -> <strong>{\"name\":\"default\",\"clientURLs\":[\"[http://localhost:2379\](http://localhost:2379\)"]}</strong></code>
    * <code>/0/members/8e9e05c52164694d/RaftAttributes -> <strong>"{\"peerURLs\":[\"http://localhost:2380\"]}"</strong></code>
* Storage version: /0/version-> 3.5.0


### Tools


#### protoc

Following command allows you to see the file content when executed from etcd root directory:


```
cat default.etcd/member/snap/0000000000000002-0000000000049425.snap | 
  protoc --decode=snappb.snapshot \
    server/etcdserver/api/snap/snappb/snap.proto \
    -I $(go list -f '{{.Dir}}' github.com/gogo/protobuf/proto)/.. \
    -I . 
    -I $(go list -m -f '{{.Dir}}' github.com/gogo/protobuf)/protobuf
```


Analogously you can extract 'data' field and decode as '[Raftpb.Snapshot](https://github.com/etcd-io/etcd/blob/ad5b30297a43daeb5ce7311fa606ce4c1f16618f/raft/raftpb/raft.proto#L31)`'`


### Exemplar JSON serialized store v2 content in etcd 3.4 *.snap files:


```
{
  "Root":{
    "Path":"/",
    "CreatedIndex":0,
    "ModifiedIndex":0,
    "ExpireTime":"0001-01-01T00:00:00Z",
    "Value":"",
    "Children":{
      "0":{
        "Path":"/0",
        "CreatedIndex":0,
        "ModifiedIndex":0,
        "ExpireTime":"0001-01-01T00:00:00Z",
        "Value":"",
        "Children":{
          "members":{
            "Path":"/0/members",
            "CreatedIndex":1,
            "ModifiedIndex":1,
            "ExpireTime":"0001-01-01T00:00:00Z",
            "Value":"",
            "Children":{
              "8e9e05c52164694d":{
                "Path":"/0/members/8e9e05c52164694d",
                "CreatedIndex":1,
                "ModifiedIndex":1,
                "ExpireTime":"0001-01-01T00:00:00Z",
                "Value":"",
                "Children":{
                  "attributes":{
                    "Path":"/0/members/8e9e05c52164694d/attributes",
                    "CreatedIndex":2,
                    "ModifiedIndex":2,
                    "ExpireTime":"0001-01-01T00:00:00Z",
                    "Value":"{\"name\":\"default\",\"clientURLs\":[\"http://localhost:2379\"]}",
                    "Children":null
                  },
                  "RaftAttributes":{
                    "Path":"/0/members/8e9e05c52164694d/RaftAttributes",
                    "CreatedIndex":1,
                    "ModifiedIndex":1,
                    "ExpireTime":"0001-01-01T00:00:00Z",
                    "Value":"{\"peerURLs\":[\"http://localhost:2380\"]}",
                    "Children":null
                  }
                }
              }
            }
          },
          "version":{
            "Path":"/0/version",
            "CreatedIndex":3,
            "ModifiedIndex":3,
            "ExpireTime":"0001-01-01T00:00:00Z",
            "Value":"3.5.0",
            "Children":null
          }
        }
      },
      "1":{
        "Path":"/1",
        "CreatedIndex":0,
        "ModifiedIndex":0,
        "ExpireTime":"0001-01-01T00:00:00Z",
        "Value":"",
        "Children":{


        }
      }
    }
  },
  "WatcherHub":{
    "EventHistory":{
      "Queue":{
        "Events":[
          {
            "action":"create",
            "node":{
              "key":"/0/members/8e9e05c52164694d/RaftAttributes",
              "value":"{\"peerURLs\":[\"http://localhost:2380\"]}",
              "modifiedIndex":1,
              "createdIndex":1
            }
          },
          {
            "action":"set",
            "node":{
              "key":"/0/members/8e9e05c52164694d/attributes",
              "value":"{\"name\":\"default\",\"clientURLs\":[\"http://localhost:2379\"]}",
              "modifiedIndex":2,
              "createdIndex":2
            }
          },
          {
            "action":"set",
            "node":{
              "key":"/0/version",
              "value":"3.5.0",
              "modifiedIndex":3,
              "createdIndex":3
            }
          }
        ]
      }
    }
  }
}





<!-- Footnotes themselves at the bottom. -->
## Notes

[^1]:
     The metadata pages at the beginning of the bbolt file are modified in-place.

[^2]:

     Inconsistent, as majority of uint’s are written bigendian

[^3]:
     The initial (index:0) snapshot at the beginning of WAL log is not associated with *.snap file. Also the old *.snap files (or WAL logs) might get purged.


## Changes

This section is reserved to describe changes to the file formats introduces 
between different etcd versions.
