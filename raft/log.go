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
	"fmt"
	"log"

	pb "go.etcd.io/etcd/raft/v3/raftpb"
)

// Raft 协议中日志复制部分的核心就是在集群中各个节点之间完成日志的复制，因此在 etcd-raft模块的实现中使用 raftLog结构来管理节点上的日志
// 它依赖于前面介绍的 Storage接口和unstable结构体
type raftLog struct {
	// storage contains all stable entries since the last snapshot.
	// 实际上就是前面介绍的 MemoryStorage 实例，其中存储了快照数据及该快照之后的 Entry 记录。
	// 在有的文档中，也将该 MemoryStorage 实例称为“stable storage”。
	storage Storage

	// unstable contains all unstable entries and snapshot.
	// they will be saved into storage.
	// 用于存储未写入 Storage 的快照数据及 Entry 记录
	unstable unstable

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	// 已提交的位置，即已提交的 Entry 记录中最大的索引值。
	committed uint64
	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	// 已应用 的位置，即已应用的 Entry记录中最大的索引值
	applied uint64

	logger Logger

	// maxNextEntsSize is the maximum number aggregate byte size of the messages
	// returned from calls to nextEnts.
	maxNextEntsSize uint64
}

// newLog returns log using the given storage and default options. It
// recovers the log to the state that it just commits and applies the
// latest snapshot.
func newLog(storage Storage, logger Logger) *raftLog {
	return newLogWithSize(storage, logger, noLimit)
}

// newLogWithSize returns a log using the given storage and max
// message size.
func newLogWithSize(storage Storage, logger Logger, maxNextEntsSize uint64) *raftLog {
	if storage == nil {
		log.Panic("storage must not be nil")
	}
	log := &raftLog{
		storage:         storage,
		logger:          logger,
		maxNextEntsSize: maxNextEntsSize,
	}
	// 获取 Storage 中的第一条Entry和最后一条Entry的索引值
	firstIndex, err := storage.FirstIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	lastIndex, err := storage.LastIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	// 初始化unstable.offset
	log.unstable.offset = lastIndex + 1
	log.unstable.logger = logger
	// Initialize our committed and applied pointers to the time of the last compaction.
	// 初始化 committed、 applied 字段
	log.committed = firstIndex - 1
	log.applied = firstIndex - 1

	return log
}

func (l *raftLog) String() string {
	return fmt.Sprintf("committed=%d, applied=%d, unstable.offset=%d, len(unstable.Entries)=%d", l.committed, l.applied, l.unstable.offset, len(l.unstable.entries))
}

// maybeAppend returns (0, false) if the entries cannot be appended. Otherwise,
// it returns (last index of new entries, true).
// 当 Follower 节点或 Candidate 节点需要向 raftLog 中追加 Entry 记录时
// 会通过 raft.handleAppendEntries() 方法调用 raftLog.maybeAppend()方法完成追加 Entry记录的功能。
// index: MsgApp 消息携带的第一条 Entry 的 Index值，这里的MsgApp消息是etcd-raft模块中定义的消息类型之一， 对应Raft协议中提到的Append Entries消息;
// logTerm: 是MsgApp消息的LogTerm宇段，通过消息中携带的该Term值与当前节点记录的Term进行比较，就可以判断消息是否为过时的消息
// committed: 是MsgApp消息的Commit宇段，Leader节点通过该字段通知 Follower节点当前己提交 Entry 的位置
// ents: 是MsgApp消息中携带的 Entry记录，即待追加到raftLog中的Entry记录。
func (l *raftLog) maybeAppend(index, logTerm, committed uint64, ents ...pb.Entry) (lastnewi uint64, ok bool) {
	if l.matchTerm(index, logTerm) { // matchTerm()方法检测 MsgApp 消息的 Index 字段及 LogTerm 字段是否合法
		lastnewi = index + uint64(len(ents))
		ci := l.findConflict(ents)
		fmt.Printf("raftLog.maybeAppend, lastnewi=%d, ci=%d \n", lastnewi, ci)
		switch {
		case ci == 0:
			// findConflict()方法返回0时，表示raftLog中已经包含了所有待追加的 Entry 记录，不必进行任何追加操作
		case ci <= l.committed:
			//  如出现冲突的位置是已提交的记录，则输出异常日志并终止整个程序
			l.logger.Panicf("entry %d conflict with committed entry [committed(%d)]", ci, l.committed)
		default:
			// 如果冲突位置是未提交的部分
			offset := index + 1

			// 则将ents中未发生冲突的部分追加到raftLog中
			l.append(ents[ci-offset:]...)
		}
		l.commitTo(min(committed, lastnewi))
		return lastnewi, true
	}
	return 0, false
}

func (l *raftLog) append(ents ...pb.Entry) uint64 {
	if len(ents) == 0 {
		return l.lastIndex()
	}
	if after := ents[0].Index - 1; after < l.committed {
		l.logger.Panicf("after(%d) is out of range [committed(%d)]", after, l.committed)
	}
	// 将 Entry 记录追加到 unstable 中
	l.unstable.truncateAndAppend(ents)
	return l.lastIndex() // 返回 raftLog 最后一条日志记录的索引
}

// findConflict finds the index of the conflict.
// It returns the first pair of conflicting entries between the existing
// entries and the given entries, if there are any.
// If there is no conflicting entries, and the existing entries contains
// all the given entries, zero will be returned.
// If there is no conflicting entries, but the given entries contains new
// entries, the index of the first new entry will be returned.
// An entry is considered to be conflicting if it has the same index but
// a different term.
// The index of the given entries MUST be continuously increasing.
func (l *raftLog) findConflict(ents []pb.Entry) uint64 {
	for _, ne := range ents {
		// 追历全部待追加的 Entry， 判断 raftLog 中是否存在冲突的 Entry 记录
		if !l.matchTerm(ne.Index, ne.Term) { // 查找冲突的 Entry记录
			if ne.Index <= l.lastIndex() {
				l.logger.Infof("found conflict at index %d [existing term: %d, conflicting term: %d]",
					ne.Index, l.zeroTermOnErrCompacted(l.term(ne.Index)), ne.Term)
			}

			// 返回冲突记录的索引值
			return ne.Index
		}
	}
	return 0
}

// findConflictByTerm takes an (index, term) pair (indicating a conflicting log
// entry on a leader/follower during an append) and finds the largest index in
// log l with a term <= `term` and an index <= `index`. If no such index exists
// in the log, the log's first index is returned.
//
// The index provided MUST be equal to or less than l.lastIndex(). Invalid
// inputs log a warning and the input index is returned.
func (l *raftLog) findConflictByTerm(index uint64, term uint64) uint64 {
	if li := l.lastIndex(); index > li {
		// NB: such calls should not exist, but since there is a straightfoward
		// way to recover, do it.
		//
		// It is tempting to also check something about the first index, but
		// there is odd behavior with peers that have no log, in which case
		// lastIndex will return zero and firstIndex will return one, which
		// leads to calls with an index of zero into this method.
		l.logger.Warningf("index(%d) is out of range [0, lastIndex(%d)] in findConflictByTerm",
			index, li)
		return index
	}
	for {
		logTerm, err := l.term(index)
		if logTerm <= term || err != nil {
			break
		}
		index--
	}
	return index
}

func (l *raftLog) unstableEntries() []pb.Entry {
	if len(l.unstable.entries) == 0 {
		return nil
	}
	return l.unstable.entries
}

// nextEnts returns all the available entries for execution.
// If applied is smaller than the index of snapshot, it returns all committed
// entries after the index of snapshot.
// 当上层模块需要从 raftLog获取 Entry 记录进行处理时，会先调用 hasNextEnts()方法检测是否有待应用的记录
// 然后调用 nextEnts()方法将己提交且未应用的 Entry记录返回给上层模块处理
func (l *raftLog) nextEnts() (ents []pb.Entry) {
	fmt.Printf("raftLog.nextEnts, l.applied+1=%d, l.firstIndex()=%d, l.committed+1=%d \n", l.applied+1, l.firstIndex(), l.committed+1)
	off := max(l.applied+1, l.firstIndex()) // 获取当前已经应用记录的位置
	if l.committed+1 > off {                // 是否存在 已提交且未应用的 Entry 记录
		// 获取全部已提交且未应用的 Entry 记录并返回

		unb, _ := json.Marshal(l.unstable)
		sb, _ := json.Marshal(l.storage)

		fmt.Println("raftLog.nextEnts, l.unstable=", string(unb))
		fmt.Println("raftLog.nextEnts, l.storage=", string(sb))

		ents, err := l.slice(off, l.committed+1, l.maxNextEntsSize)
		if err != nil {
			l.logger.Panicf("unexpected error when getting unapplied entries (%v)", err)
		}
		return ents
	}
	return nil
}

// hasNextEnts returns if there is any available entries for execution. This
// is a fast check without heavy raftLog.slice() in raftLog.nextEnts().
func (l *raftLog) hasNextEnts() bool {
	off := max(l.applied+1, l.firstIndex())
	return l.committed+1 > off
}

// hasPendingSnapshot returns if there is pending snapshot waiting for applying.
func (l *raftLog) hasPendingSnapshot() bool {
	return l.unstable.snapshot != nil && !IsEmptySnap(*l.unstable.snapshot)
}

func (l *raftLog) snapshot() (pb.Snapshot, error) {
	if l.unstable.snapshot != nil {
		return *l.unstable.snapshot, nil
	}
	return l.storage.Snapshot()
}

// 先会去 unstable 中查找相应的 Entry记录，如果查找不到，则再去 storage 中查找
func (l *raftLog) firstIndex() uint64 {
	if i, ok := l.unstable.maybeFirstIndex(); ok {
		fmt.Println("raftLog.firstIndex, l.unstable.maybeFirstIndex() is ok, i(firstIndex)=", i)
		return i
	}
	index, err := l.storage.FirstIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	fmt.Println("raftLog.firstIndex, l.storage.FirstIndex() is ok, index(firstIndex)=", index)
	return index
}

func (l *raftLog) lastIndex() uint64 {
	if i, ok := l.unstable.maybeLastIndex(); ok {
		return i
	}
	i, err := l.storage.LastIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	return i
}

func (l *raftLog) commitTo(tocommit uint64) {
	// never decrease commit
	if l.committed < tocommit {
		if l.lastIndex() < tocommit {
			l.logger.Panicf("tocommit(%d) is out of range [lastIndex(%d)]. Was the raft log corrupted, truncated, or lost?", tocommit, l.lastIndex())
		}
		l.committed = tocommit
	}
}

func (l *raftLog) appliedTo(i uint64) {
	if i == 0 {
		return
	}
	if l.committed < i || i < l.applied {
		l.logger.Panicf("applied(%d) is out of range [prevApplied(%d), committed(%d)]", i, l.applied, l.committed)
	}
	// raftLog.applied的更新
	l.applied = i
}

func (l *raftLog) stableTo(i, t uint64) { l.unstable.stableTo(i, t) }

func (l *raftLog) stableSnapTo(i uint64) { l.unstable.stableSnapTo(i) }

func (l *raftLog) lastTerm() uint64 {
	t, err := l.term(l.lastIndex())
	if err != nil {
		l.logger.Panicf("unexpected error when getting the last term (%v)", err)
	}
	return t
}

// 查找指定索引值对应的Term值
func (l *raftLog) term(i uint64) (uint64, error) {
	// 先会去 unstable 中查找相应的 Entry记录，如果查找不到，则再去 storage 中查找
	// the valid term range is [index of dummy entry, last index]
	dummyIndex := l.firstIndex() - 1
	if i < dummyIndex || i > l.lastIndex() {
		// TODO: return an error instead?
		return 0, nil
	}

	// 尝试从 unstable 中获取对应的 Entry记录并返回其 Term值
	if t, ok := l.unstable.maybeTerm(i); ok {
		return t, nil
	}

	// 尝试从 storage 中 获取对应的 Entry 记录并返回其 Term 值
	t, err := l.storage.Term(i)
	if err == nil {
		return t, nil
	}
	if err == ErrCompacted || err == ErrUnavailable {
		return 0, err
	}
	panic(err) // TODO(bdarnell)
}

// 在 Leader 节点向 Follower 节 点发送 MsgApp 消息 时， 需要根 据 Follower 节 点的日志复制 (NextIndex 和 MatchIndex)情况决定发送的 Entry 记录
// 此时 需要调用 raftLog.entries()方法获取指定的 Entry记录
func (l *raftLog) entries(i, maxsize uint64) ([]pb.Entry, error) {
	if i > l.lastIndex() {
		return nil, nil
	}
	return l.slice(i, l.lastIndex()+1, maxsize)
}

// allEntries returns all entries in the log.
func (l *raftLog) allEntries() []pb.Entry {
	ents, err := l.entries(l.firstIndex(), noLimit)
	if err == nil {
		return ents
	}
	if err == ErrCompacted { // try again if there was a racing compaction
		return l.allEntries()
	}
	// TODO (xiangli): handle error?
	panic(err)
}

// isUpToDate determines if the given (lastIndex,term) log is more up-to-date
// by comparing the index and term of the last entries in the existing logs.
// If the logs have last entries with different terms, then the log with the
// later term is more up-to-date. If the logs end with the same term, then
// whichever log has the larger lastIndex is more up-to-date. If the logs are
// the same, the given log is up-to-date.
func (l *raftLog) isUpToDate(lasti, term uint64) bool {
	return term > l.lastTerm() || (term == l.lastTerm() && lasti >= l.lastIndex())
}

func (l *raftLog) matchTerm(i, term uint64) bool {
	// 查询指定索引值对应的 Entry记录的 Term值
	t, err := l.term(i)
	if err != nil {
		return false
	}
	return t == term
}

func (l *raftLog) maybeCommit(maxIndex, term uint64) bool {
	if maxIndex > l.committed && l.zeroTermOnErrCompacted(l.term(maxIndex)) == term {
		l.commitTo(maxIndex)
		return true
	}
	return false
}

func (l *raftLog) restore(s pb.Snapshot) {
	l.logger.Infof("log [%s] starts to restore snapshot [index: %d, term: %d]", l, s.Metadata.Index, s.Metadata.Term)
	l.committed = s.Metadata.Index
	l.unstable.restore(s)
}

// slice returns a slice of log entries from lo through hi-1, inclusive.
func (l *raftLog) slice(lo, hi, maxSize uint64) ([]pb.Entry, error) {
	err := l.mustCheckOutOfBounds(lo, hi)
	if err != nil {
		return nil, err
	}
	if lo == hi {
		return nil, nil
	}
	var ents []pb.Entry
	if lo < l.unstable.offset {
		// 从 Storage 中获取记录，可能只能从 Storage 中取前半部分(也就是 lo~l.unstable.offset 部分)
		// 注意 maxSize 参数， 它会限制获取的记录切片的总字节数
		storedEnts, err := l.storage.Entries(lo, min(hi, l.unstable.offset), maxSize)
		if err == ErrCompacted {
			return nil, err
		} else if err == ErrUnavailable {
			l.logger.Panicf("entries[%d:%d) is unavailable from storage", lo, min(hi, l.unstable.offset))
		} else if err != nil {
			panic(err) // TODO(bdarnell)
		}

		// check if ents has reached the size limitation
		if uint64(len(storedEnts)) < min(hi, l.unstable.offset)-lo {
			return storedEnts, nil
		}

		ents = storedEnts
	}

	// 从 unstable 中取出后半部分记录，即 l.unstable.offset~hi 的日志记录
	if hi > l.unstable.offset {
		unstable := l.unstable.slice(max(lo, l.unstable.offset), hi)
		if len(ents) > 0 {
			combined := make([]pb.Entry, len(ents)+len(unstable))
			n := copy(combined, ents)
			copy(combined[n:], unstable)
			ents = combined
		} else {
			ents = unstable
		}
	}
	return limitSize(ents, maxSize), nil
}

// l.firstIndex <= lo <= hi <= l.firstIndex + len(l.entries)
func (l *raftLog) mustCheckOutOfBounds(lo, hi uint64) error {
	if lo > hi {
		l.logger.Panicf("invalid slice %d > %d", lo, hi)
	}
	fi := l.firstIndex()
	if lo < fi {
		return ErrCompacted
	}

	length := l.lastIndex() + 1 - fi
	if hi > fi+length {
		l.logger.Panicf("slice[%d,%d) out of bound [%d,%d]", lo, hi, fi, l.lastIndex())
	}
	return nil
}

func (l *raftLog) zeroTermOnErrCompacted(t uint64, err error) uint64 {
	if err == nil {
		return t
	}
	if err == ErrCompacted {
		return 0
	}
	l.logger.Panicf("unexpected error (%v)", err)
	return 0
}
