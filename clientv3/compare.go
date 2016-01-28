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

package clientv3

import (
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type CompareTarget int
type CompareResult int

const (
	CompareVersion CompareTarget = iota
	CompareCreated
	CompareModified
	CompareValue
)

type Cmp pb.Compare

func Compare(key string, t pb.Compare_CompareTarget, result string, v interface{}) Cmp {
	var r pb.Compare_CompareResult

	switch result {
	case "=":
		r = pb.Compare_EQUAL
	case ">":
		r = pb.Compare_GREATER
	case "<":
		r = pb.Compare_LESS
	default:
		panic("Unknown result op")
	}

	switch t {
	case pb.Compare_VALUE:
		val, ok := v.(string)
		if !ok {
			panic("bad compare value")
		}
		return Cmp{Key: []byte(key), Result: r, Target: t, TargetUnion: &pb.Compare_Value{Value: []byte(val)}}
	case pb.Compare_VERSION:
		return Cmp{Key: []byte(key), Result: r, Target: t, TargetUnion: &pb.Compare_Version{Version: mustInt64(v)}}
	case pb.Compare_CREATE:
		return Cmp{Key: []byte(key), Result: r, Target: t, TargetUnion: &pb.Compare_CreateRevision{CreateRevision: mustInt64(v)}}
	case pb.Compare_MOD:
		return Cmp{Key: []byte(key), Result: r, Target: t, TargetUnion: &pb.Compare_ModRevision{ModRevision: mustInt64(v)}}
	default:
		panic("Unknown compare type")
	}
}

func Value(key string) (string, pb.Compare_CompareTarget) {
	return key, pb.Compare_VALUE
}

func Version(key string) (string, pb.Compare_CompareTarget) {
	return key, pb.Compare_VERSION
}

func CreatedRevision(key string) (string, pb.Compare_CompareTarget) {
	return key, pb.Compare_CREATE
}

func ModifiedRevision(key string) (string, pb.Compare_CompareTarget) {
	return key, pb.Compare_MOD
}

func mustInt64(val interface{}) int64 {
	if v, ok := val.(int64); ok {
		return v
	}
	if v, ok := val.(int); ok {
		return int64(v)
	}
	panic("bad value")
}
