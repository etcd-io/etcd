// Copyright 2016 The etcd Authors
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

// Package v3alarm manages health status alarms in etcd.
package v3alarm

import (
	"sync"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"

	"go.uber.org/zap"
)

type BackendGetter interface {
	Backend() backend.Backend
}

type alarmSet map[types.ID]*pb.AlarmMember

// AlarmStore 与 quotaApplierV3 配合实现限流功能
// 在前面介绍 quotaApplierV3 时我们提到，当首次触发限流时会创建 AlarmRequest 请求并封 装成 MsgProp 消息发送到集群 当中 ，在 AlarmRequest 中封装的信息如下所示 。
// AlarmStore persists alarms to the backend.
type AlarmStore struct {
	lg *zap.Logger
	mu sync.Mutex

	// 在该 map 字段中记录了每种 AlarmType 对应的 AlarmMember 实例。
	// AlarmType 现在只有 AlarmType_NONE 和 AlarmType_NOSPACE 两种类型
	// alarmSet类型实际上是 map[types.ID]*pb.AlarmMember类型，其中记录了节点 ID 与 AlarmMember之间的映射关系。
	types map[pb.AlarmType]alarmSet

	// BackendGetter 接口用于返回该 AlarmStore 实例使用的存储。
	// EtcdServer 就是 BackendGetter 接口的实现之一，返回的就是其底层使用的 backend 实例。
	bg BackendGetter
}

func NewAlarmStore(lg *zap.Logger, bg BackendGetter) (*AlarmStore, error) {
	if lg == nil {
		lg = zap.NewNop()
	}
	ret := &AlarmStore{lg: lg, types: make(map[pb.AlarmType]alarmSet), bg: bg}
	err := ret.restore()
	return ret, err
}

func (a *AlarmStore) Activate(id types.ID, at pb.AlarmType) *pb.AlarmMember {
	a.mu.Lock()
	defer a.mu.Unlock()

	newAlarm := &pb.AlarmMember{MemberID: uint64(id), Alarm: at}
	if m := a.addToMap(newAlarm); m != newAlarm {
		return m
	}

	v, err := newAlarm.Marshal()
	if err != nil {
		a.lg.Panic("failed to marshal alarm member", zap.Error(err))
	}

	b := a.bg.Backend()
	b.BatchTx().Lock()
	b.BatchTx().UnsafePut(buckets.Alarm, v, nil)
	b.BatchTx().Unlock()

	return newAlarm
}

func (a *AlarmStore) Deactivate(id types.ID, at pb.AlarmType) *pb.AlarmMember {
	a.mu.Lock()
	defer a.mu.Unlock()

	t := a.types[at]
	if t == nil {
		t = make(alarmSet)
		a.types[at] = t
	}
	m := t[id]
	if m == nil {
		return nil
	}

	delete(t, id)

	v, err := m.Marshal()
	if err != nil {
		a.lg.Panic("failed to marshal alarm member", zap.Error(err))
	}

	b := a.bg.Backend()
	b.BatchTx().Lock()
	b.BatchTx().UnsafeDelete(buckets.Alarm, v)
	b.BatchTx().Unlock()

	return m
}

func (a *AlarmStore) Get(at pb.AlarmType) (ret []*pb.AlarmMember) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if at == pb.AlarmType_NONE {
		for _, t := range a.types {
			for _, m := range t {
				ret = append(ret, m)
			}
		}
		return ret
	}
	for _, m := range a.types[at] {
		ret = append(ret, m)
	}
	return ret
}

func (a *AlarmStore) restore() error {
	b := a.bg.Backend()
	tx := b.BatchTx()

	tx.Lock()
	tx.UnsafeCreateBucket(buckets.Alarm)
	err := tx.UnsafeForEach(buckets.Alarm, func(k, v []byte) error {
		var m pb.AlarmMember
		if err := m.Unmarshal(k); err != nil {
			return err
		}
		a.addToMap(&m)
		return nil
	})
	tx.Unlock()

	b.ForceCommit()
	return err
}

func (a *AlarmStore) addToMap(newAlarm *pb.AlarmMember) *pb.AlarmMember {
	t := a.types[newAlarm.Alarm]
	if t == nil {
		t = make(alarmSet)
		a.types[newAlarm.Alarm] = t
	}
	m := t[types.ID(newAlarm.MemberID)]
	if m != nil {
		return m
	}
	t[types.ID(newAlarm.MemberID)] = newAlarm
	return newAlarm
}
