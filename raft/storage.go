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

package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	pb "go.etcd.io/etcd/raft/v3/raftpb"
)

// ErrCompacted is returned by Storage.Entries/Compact when a requested
// index is unavailable because it predates the last snapshot.
var ErrCompacted = errors.New("requested index is unavailable due to compaction")

// ErrSnapOutOfDate is returned by Storage.CreateSnapshot when a requested
// index is older than the existing snapshot.
var ErrSnapOutOfDate = errors.New("requested index is older than the existing snapshot")

// ErrUnavailable is returned by Storage interface when the requested log entries
// are unavailable.
var ErrUnavailable = errors.New("requested entry at index is unavailable")

// ErrSnapshotTemporarilyUnavailable is returned by the Storage interface when the required
// snapshot is temporarily unavailable.
var ErrSnapshotTemporarilyUnavailable = errors.New("snapshot is temporarily unavailable")

// Storage is an interface that may be implemented by the application
// to retrieve log entries from storage.
//
// If any Storage method returns an error, the raft instance will
// become inoperable and refuse to participate in elections; the
// application is responsible for cleanup and recovery in this case.
// 主要作用就是存储当前节点接收到的 Entry记录
type Storage interface {
	// TODO(tbg): split this into two interfaces, LogStorage and StateStorage.

	// InitialState returns the saved HardState and ConfState information.
	// 返回 Storage 中记录的状态信息，返回的是 HardState 实例和 ConfState 实例
	// 在前面介绍 Raft 协议时提到，集群中每个节点都需要保存一些必需的基本信息，在 etcd 中将其封装成 HardState，
	// 其中主要封装了当前任期号(Term 字段)、当前节点在该任期中将选票投给了哪个节点( Vote 字段)、
	// 已提交 Entry记录的位置( Commit 字段，即最后一条己提交记录的索引值)
	// ConfState 中封装了当前集群中所有节点的 ID (Nodes 字段)
	InitialState() (pb.HardState, pb.ConfState, error)
	// Entries returns a slice of log entries in the range [lo,hi).
	// MaxSize limits the total size of the log entries returned, but
	// Entries returns at least one entry if any.
	// 在 Storage 中记录了当前节点的所有 Entry 记录， Entries 方法返回指定范固的 Entry 记录(lo, hi),
	// 第三个参数{maxSize} 限定了返回的 Entry 集合的字节数上限
	Entries(lo, hi, maxSize uint64) ([]pb.Entry, error)
	// Term returns the term of entry i, which must be in the range
	// [FirstIndex()-1, LastIndex()]. The term of the entry before
	// FirstIndex is retained for matching purposes even though the
	// rest of that entry may not be available.
	// 查询指定 Index 对应的 Entry 的 Term 值
	Term(i uint64) (uint64, error)
	// LastIndex returns the index of the last entry in the log.
	// 该方法返回Storage中记录的最后一条Entry的索引值(Index)
	LastIndex() (uint64, error)
	// FirstIndex returns the index of the first log entry that is
	// possibly available via Entries (older entries have been incorporated
	// into the latest Snapshot; if storage only contains the dummy entry the
	// first log entry is not available).
	// 该方法返回Storage中记录的第一条Entry的索引值(Index)，在该Entry之前的所有Entry都已经被包含进了最近的一次 Snapshot 中
	FirstIndex() (uint64, error)
	// Snapshot returns the most recent snapshot.
	// If snapshot is temporarily unavailable, it should return ErrSnapshotTemporarilyUnavailable,
	// so raft state machine could know that Storage needs some time to prepare
	// snapshot and call Snapshot later.
	// 返回最近一次生成的快照数据
	Snapshot() (pb.Snapshot, error)
}

// MemoryStorage implements the Storage interface backed by an
// in-memory array.
type MemoryStorage struct {
	// Protects access to all fields. Most methods of MemoryStorage are
	// run on the raft goroutine, but Append() is run on an application
	// goroutine.
	sync.Mutex

	hardState pb.HardState
	snapshot  pb.Snapshot
	// ents[i] has raft log position i+snapshot.Metadata.Index
	ents []pb.Entry
}

// NewMemoryStorage creates an empty MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		// When starting from scratch populate the list with a dummy entry at term zero.
		ents: make([]pb.Entry, 1),
	}
}

// InitialState implements the Storage interface.
func (ms *MemoryStorage) InitialState() (pb.HardState, pb.ConfState, error) {
	return ms.hardState, ms.snapshot.Metadata.ConfState, nil
}

// SetHardState saves the current HardState.
func (ms *MemoryStorage) SetHardState(st pb.HardState) error {
	ms.Lock()
	defer ms.Unlock()
	ms.hardState = st
	return nil
}

// Entries implements the Storage interface.
func (ms *MemoryStorage) Entries(lo, hi, maxSize uint64) ([]pb.Entry, error) {
	ms.Lock()
	defer ms.Unlock()
	offset := ms.ents[0].Index
	// 这里 <= 说明第一个Entry是没用的，需要找一下第一个Entry是啥
	if lo <= offset {
		// 如果待查询的最小 Index 值(参数 lo) 小于 FirstIndex，是的话直接抛出异常
		return nil, ErrCompacted
	}
	if hi > ms.lastIndex()+1 {
		// 如果待查询的最大 Index 值(参数 hi)大于 LastIndex，则直接抛出异常
		getLogger().Panicf("entries' hi(%d) is out of bound lastindex(%d)", hi, ms.lastIndex())
	}
	// only contains dummy entries.
	if len(ms.ents) == 1 {
		return nil, ErrUnavailable
	}

	// lo-offset 一定大于0
	ents := ms.ents[lo-offset : hi-offset]
	return limitSize(ents, maxSize), nil
}

// Term implements the Storage interface.
func (ms *MemoryStorage) Term(i uint64) (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	offset := ms.ents[0].Index
	if i < offset {
		return 0, ErrCompacted
	}
	if int(i-offset) >= len(ms.ents) {
		return 0, ErrUnavailable
	}
	return ms.ents[i-offset].Term, nil
}

// LastIndex implements the Storage interface.
func (ms *MemoryStorage) LastIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.lastIndex(), nil
}

func (ms *MemoryStorage) lastIndex() uint64 {
	an, _ := json.Marshal(ms.ents[0])
	fmt.Println("MemoryStorage.lastIndex, ms.ents[0]=", string(an))
	fmt.Println("MemoryStorage.lastIndex, len(ms.ents)=", len(ms.ents))

	ln, _ := json.Marshal(ms.ents[len(ms.ents)-1])
	fmt.Println("MemoryStorage.lastIndex, ms.ents[len(ms.ents)-1]=", string(ln))

	return ms.ents[0].Index + uint64(len(ms.ents)) - 1
}

// FirstIndex implements the Storage interface.
func (ms *MemoryStorage) FirstIndex() (uint64, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.firstIndex(), nil
}

func (ms *MemoryStorage) firstIndex() uint64 {
	// 如果有snapshot数据，第一个Entry的Index，是MemoryStorage.snapshot.Meta的Index
	// 如果没有snapshot数据，第一个Entry的Index是0，第一个Entry=pb.Entry{},都是默认值
	// 所以真正有效的Index是数组第一个Entry的Index+1
	an, _ := json.Marshal(ms.ents[0])
	fmt.Println("MemoryStorage.firstIndex, ms.ents[0]=", string(an))
	ln, _ := json.Marshal(ms.ents[len(ms.ents)-1])
	fmt.Println("MemoryStorage.firstIndex, ms.ents[len(ms.ents)-1]=", string(ln))

	return ms.ents[0].Index + 1
}

// Snapshot implements the Storage interface.
func (ms *MemoryStorage) Snapshot() (pb.Snapshot, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.snapshot, nil
}

// ApplySnapshot overwrites the contents of this Storage object with
// those of the given snapshot.
// 将 Snapshot实例保存到 MemoryStorage 中，在节点重启时，就会通过读取快照文件创建对应的 SnapShot 实例，然后保存到 MemoryStorage 中
func (ms *MemoryStorage) ApplySnapshot(snap pb.Snapshot) error {
	ms.Lock()
	defer ms.Unlock()

	//handle check for old snapshot being applied
	// 通过快照的元数据比较当前 MemoryStorage 中记录的 Snapshot 与待处理的 Snapshot 数据的新旧程度
	msIndex := ms.snapshot.Metadata.Index
	snapIndex := snap.Metadata.Index
	if msIndex >= snapIndex {
		return ErrSnapOutOfDate // 如果待处理 Snapshot 数据比较旧，直接抛出异常
	}

	ms.snapshot = snap
	// 重置 MemoryStorage.ents 字段，此时在 ents 中只有一个空的 Entry 实例，可以理解为ents的第一个Entry是不会用到的，不太清楚为啥这样设计
	ms.ents = []pb.Entry{{Term: snap.Metadata.Term, Index: snap.Metadata.Index}}

	mhsb, _ := json.Marshal(ms.hardState)
	mshb, _ := json.Marshal(ms.snapshot)
	mentsb, _ := json.Marshal(ms.ents)

	fmt.Printf("MemoryStorage.ApplySnapshot end, ms.hardState=%s, ms.snapshot=%s, ms.ents=%s \n", string(mhsb), string(mshb), string(mentsb))

	return nil
}

// CreateSnapshot makes a snapshot which can be retrieved with Snapshot() and
// can be used to reconstruct the state at that point.
// If any configuration changes have been made since the last compaction,
// the result of the last ApplyConfChange must be passed in.
// i 是新建 Snapshot 包含的最大的索引值
// cs 是当前集群的状态，
// data 是新建Snapshot的具体数据
func (ms *MemoryStorage) CreateSnapshot(i uint64, cs *pb.ConfState, data []byte) (pb.Snapshot, error) {
	ms.Lock()
	defer ms.Unlock()
	if i <= ms.snapshot.Metadata.Index {
		// i 必须大于当前 Snapshot 包含的最大 Index 值
		return pb.Snapshot{}, ErrSnapOutOfDate
	}

	offset := ms.ents[0].Index
	if i > ms.lastIndex() {
		// i 必须 小于 MemoryStorage 的 LastIndex 值，
		getLogger().Panicf("snapshot %d is out of bound lastindex(%d)", i, ms.lastIndex())
	}

	// 更新 MemoryStorage.snapshot 的元数据
	ms.snapshot.Metadata.Index = i
	ms.snapshot.Metadata.Term = ms.ents[i-offset].Term
	if cs != nil {
		ms.snapshot.Metadata.ConfState = *cs
	}
	ms.snapshot.Data = data // 更新具体的快照数据
	return ms.snapshot, nil
}

// Compact discards all log entries prior to compactIndex.
// It is the application's responsibility to not attempt to compact an index
// greater than raftLog.applied.
// 新建 Snapshot 之后，一般会调用 MemoryStorage.Compact()方法将 MemoryStorage.ents 中指定索引之前的 Entry记录全部抛弃，
// 从而实现压缩 MemoryStorage.ents 的目的
func (ms *MemoryStorage) Compact(compactIndex uint64) error {
	ms.Lock()
	defer ms.Unlock()
	offset := ms.ents[0].Index
	if compactIndex <= offset {
		return ErrCompacted
	}
	if compactIndex > ms.lastIndex() {
		getLogger().Panicf("compact %d is out of bound lastindex(%d)", compactIndex, ms.lastIndex())
	}

	i := compactIndex - offset
	ents := make([]pb.Entry, 1, 1+uint64(len(ms.ents))-i)
	ents[0].Index = ms.ents[i].Index
	ents[0].Term = ms.ents[i].Term
	ents = append(ents, ms.ents[i+1:]...)
	ms.ents = ents
	return nil
}

// Append the new entries to storage.
// TODO (xiangli): ensure the entries are continuous and
// entries[0].Index > ms.entries[0].Index
// 设置完快照数据之后，就可以开始向 MemoryStorage 中追加 Entry 记录了
func (ms *MemoryStorage) Append(entries []pb.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	ms.Lock()
	defer ms.Unlock()

	re, _ := json.Marshal(entries)

	mhsb, _ := json.Marshal(ms.hardState)
	mshb, _ := json.Marshal(ms.snapshot)
	mentsb, _ := json.Marshal(ms.ents)

	fmt.Printf("MemoryStorage.Append start, reqEntries=%s \n", string(re))
	fmt.Printf("MemoryStorage.Append start, ms.hardState=%s, ms.snapshot=%s, ms.ents=%s \n", string(mhsb), string(mshb), string(mentsb))

	first := ms.firstIndex()

	// 获取待添加的最后一条Entry的 Index 值
	last := entries[0].Index + uint64(len(entries)) - 1

	// shortcut if there is no new entry.
	if last < first {
		// entries 切片中所有的 Entry 都已经过时，无须添加任何 Entry
		return nil
	}

	// truncate compacted entries
	// first 之前的 Entry 已经记入 Snapshot 中，不应该再记录到 ents 中，所以将这部分 Entry 截调
	if first > entries[0].Index {
		entries = entries[first-entries[0].Index:]
	}

	// 计算 entries 切片中第一条可用的 Entry 与 first 之间的差距
	offset := entries[0].Index - ms.ents[0].Index
	switch {
	case uint64(len(ms.ents)) > offset:
		// 保留 MemoryStorage.ents 中 first~offset 的部分，offset 之后的部分被抛弃
		ms.ents = append([]pb.Entry{}, ms.ents[:offset]...)
		// 然后将待追加的 Entry 追加到 MemoryStorage.ents 中
		ms.ents = append(ms.ents, entries...)
	case uint64(len(ms.ents)) == offset:
		// 直接将待追加的日志记录(entries)追加到 MemoryStorage 中
		ms.ents = append(ms.ents, entries...)
	default:
		// 异常处理
		getLogger().Panicf("missing log entry [last: %d, append at: %d]",
			ms.lastIndex(), entries[0].Index)
	}
	return nil
}
