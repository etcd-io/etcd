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

// 当 eventQueue 被填满之后， 继续向其中添加 Event， 则会导致最先添加的 Event实例被覆盖
type eventQueue struct {
	Events   []*Event // 底层真正存储 Event实例的数组
	Size     int      // 当前 Events字段中存储的 Event实例个数
	Front    int      // eventQueue 队列中第一个元素的下标值
	Back     int      // eventQueue 巳队列中最后一个元素的下标值
	Capacity int      // Events宇段的长度
}

func (eq *eventQueue) insert(e *Event) {
	eq.Events[eq.Back] = e
	eq.Back = (eq.Back + 1) % eq.Capacity // 后移 Back

	if eq.Size == eq.Capacity { //dequeue
		// 当队列满了的时候， 则后移 Front， 抛弃 Front指向的 Event 实例
		eq.Front = (eq.Front + 1) % eq.Capacity
	} else {
		eq.Size++
	}
}
