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

package etcdserver

import (
	"io"

	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/mvcc/backend"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// createMergedSnapshotMessage creates a snapshot message that contains: raft status (term, conf),
// a snapshot of v2 store inside raft.Snapshot as []byte, a snapshot of v3 KV in the top level message
// as ReadCloser.
// 该方法中会将 v2 版本存储和 v3 版本存储封装 成 snap.Message实例，
func (s *EtcdServer) createMergedSnapshotMessage(m raftpb.Message, snapt, snapi uint64, confState raftpb.ConfState) snap.Message {
	lg := s.Logger()
	// get a snapshot of v2 store as []byte
	clone := s.v2store.Clone() // 复制一份v2存储的数据， 并转换成JSON格式
	d, err := clone.SaveNoCopy()
	if err != nil {
		lg.Panic("failed to save v2 store data", zap.Error(err))
	}

	// commit kv to write metadata(for example: consistent index).
	s.KV().Commit() //提交v3存储中当前的读写事务

	dbsnap := s.be.Snapshot() //获取 v3 存储快照，其实就是对 BoltDB 数据库进行快照
	// get a snapshot of v3 KV as readCloser
	rc := newSnapshotReaderCloser(lg, dbsnap)

	//将 v2 存储的快熙、数据和相关元数据写入 raftpb. Snapshot 实例中
	// put the []byte snapshot of store into raft snapshot and return the merged snapshot with
	// KV readCloser snapshot.
	snapshot := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			Index:     snapi,
			Term:      snapt,
			ConfState: confState,
		},
		Data: d,
	}
	m.Snapshot = snapshot //将 raftpb.Snapshot 实例封装到 MsgSnap 消息中

	//将消息 MsgSnap 消息和 v3存储中的数据封装成 snap.Message 实例返回
	return *snap.NewMessage(m, rc, dbsnap.Size())
}

func newSnapshotReaderCloser(lg *zap.Logger, snapshot backend.Snapshot) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		n, err := snapshot.WriteTo(pw)
		if err == nil {
			lg.Info(
				"sent database snapshot to writer",
				zap.Int64("bytes", n),
				zap.String("size", humanize.Bytes(uint64(n))),
			)
		} else {
			lg.Warn(
				"failed to send database snapshot to writer",
				zap.String("size", humanize.Bytes(uint64(n))),
				zap.Error(err),
			)
		}
		pw.CloseWithError(err)
		err = snapshot.Close()
		if err != nil {
			lg.Panic("failed to close database snapshot", zap.Error(err))
		}
	}()
	return pr
}
