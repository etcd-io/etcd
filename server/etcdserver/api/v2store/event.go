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

package v2store

const (
	Get              = "get"
	Create           = "create"
	Set              = "set"
	Update           = "update"
	Delete           = "delete"
	CompareAndSwap   = "compareAndSwap"
	CompareAndDelete = "compareAndDelete"
	Expire           = "expire"
)

type Event struct {
	Action    string      `json:"action"`             // 该 Event 实例对应的操作 ，可选 项有 Get、 Create、 Set、 Update、 Delete、 CompareAndSwap、 CompareAndDelete 和 Expire。
	Node      *NodeExtern `json:"node,omitempty"`     // 当前操作节点对应的 NodeExtern 实例
	PrevNode  *NodeExtern `json:"prevNode,omitempty"` // 如果是更新操作，则记录该节点之前状态对应的 NodeExtern 实例。
	EtcdIndex uint64      `json:"-"`                  // 记录操作完成之后的 CurrentIndex 值
	Refresh   bool        `json:"refresh,omitempty"`  // 如果是 Set、Update、 CompareAndSwap三种涉及值更新的操作， 则该字段都有可能被设置 true。当该字段被设置为 true 时， 表示该 Event 实例对应的 修改操作只进行刷新操作(例如，只修改了节点的过期时间)， 并没有改变节点的值， 不会触发相关的 watcher
}

func newEvent(action string, key string, modifiedIndex, createdIndex uint64) *Event {
	n := &NodeExtern{
		Key:           key,
		ModifiedIndex: modifiedIndex,
		CreatedIndex:  createdIndex,
	}

	return &Event{
		Action: action,
		Node:   n,
	}
}

func (e *Event) IsCreated() bool {
	if e.Action == Create {
		return true
	}
	return e.Action == Set && e.PrevNode == nil
}

func (e *Event) Index() uint64 {
	return e.Node.ModifiedIndex
}

func (e *Event) Clone() *Event {
	return &Event{
		Action:    e.Action,
		EtcdIndex: e.EtcdIndex,
		Node:      e.Node.Clone(),
		PrevNode:  e.PrevNode.Clone(),
	}
}

func (e *Event) SetRefresh() {
	e.Refresh = true
}
