// Copyright 2021 The etcd Authors
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

package wal

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// ReadWALVersion reads remaining entries from opened WAL and returns struct
// that implements schema.WAL interface.
func ReadWALVersion(w *WAL) (*walVersion, error) {
	_, _, ents, err := w.ReadAll()
	if err != nil {
		return nil, err
	}
	return &walVersion{entries: ents}, nil
}

type walVersion struct {
	entries []raftpb.Entry
}

// MinimalEtcdVersion returns minimal etcd able to interpret entries from  WAL log,
func (w *walVersion) MinimalEtcdVersion() *semver.Version {
	return MinimalEtcdVersion(w.entries)
}

// MinimalEtcdVersion returns minimal etcd able to interpret entries from  WAL log,
// determined by looking at entries since the last snapshot and returning the highest
// etcd version annotation from used messages, fields, enums and their values.
func MinimalEtcdVersion(ents []raftpb.Entry) *semver.Version {
	var maxVer *semver.Version
	for _, ent := range ents {
		maxVer = maxVersion(maxVer, etcdVersionFromEntry(ent))
	}
	return maxVer
}

func etcdVersionFromEntry(ent raftpb.Entry) *semver.Version {
	msgVer := etcdVersionFromMessage(proto.MessageReflect(&ent))
	dataVer := etcdVersionFromData(ent.Type, ent.Data)
	return maxVersion(msgVer, dataVer)
}

func etcdVersionFromData(entryType raftpb.EntryType, data []byte) *semver.Version {
	var msg protoreflect.Message
	var ver *semver.Version
	switch entryType {
	case raftpb.EntryNormal:
		var raftReq etcdserverpb.InternalRaftRequest
		err := pbutil.Unmarshaler(&raftReq).Unmarshal(data)
		if err != nil {
			return nil
		}
		msg = proto.MessageReflect(&raftReq)
		if raftReq.ClusterVersionSet != nil {
			ver, err = semver.NewVersion(raftReq.ClusterVersionSet.Ver)
			if err != nil {
				panic(err)
			}
		}
	case raftpb.EntryConfChange:
		var confChange raftpb.ConfChange
		err := pbutil.Unmarshaler(&confChange).Unmarshal(data)
		if err != nil {
			return nil
		}
		msg = proto.MessageReflect(&confChange)
	case raftpb.EntryConfChangeV2:
		var confChange raftpb.ConfChangeV2
		err := pbutil.Unmarshaler(&confChange).Unmarshal(data)
		if err != nil {
			return nil
		}
		msg = proto.MessageReflect(&confChange)
	default:
		panic("unhandled")
	}
	return maxVersion(etcdVersionFromMessage(msg), ver)
}

func etcdVersionFromMessage(m protoreflect.Message) *semver.Version {
	var maxVer *semver.Version
	md := m.Descriptor()
	opts := md.Options().(*descriptorpb.MessageOptions)
	if opts != nil {
		ver, _ := EtcdVersionFromOptionsString(opts.String())
		maxVer = maxVersion(maxVer, ver)
	}

	m.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		fd := md.Fields().Get(field.Index())
		maxVer = maxVersion(maxVer, etcdVersionFromField(fd))
		switch m := value.Interface().(type) {
		case protoreflect.Message:
			maxVer = maxVersion(maxVer, etcdVersionFromMessage(m))
		case protoreflect.EnumNumber:
			maxVer = maxVersion(maxVer, etcdVersionFromEnum(field.Enum(), m))
		}
		return true
	})
	return maxVer
}

func etcdVersionFromEnum(enum protoreflect.EnumDescriptor, value protoreflect.EnumNumber) *semver.Version {
	var maxVer *semver.Version
	enumOpts := enum.Options().(*descriptorpb.EnumOptions)
	if enumOpts != nil {
		ver, _ := EtcdVersionFromOptionsString(enumOpts.String())
		maxVer = maxVersion(maxVer, ver)
	}
	valueDesc := enum.Values().Get(int(value))
	valueOpts := valueDesc.Options().(*descriptorpb.EnumValueOptions)
	if valueOpts != nil {
		ver, _ := EtcdVersionFromOptionsString(valueOpts.String())
		maxVer = maxVersion(maxVer, ver)
	}
	return maxVer
}

func maxVersion(a *semver.Version, b *semver.Version) *semver.Version {
	if a != nil && (b == nil || b.LessThan(*a)) {
		return a
	}
	return b
}

func etcdVersionFromField(fd protoreflect.FieldDescriptor) *semver.Version {
	opts := fd.Options().(*descriptorpb.FieldOptions)
	if opts == nil {
		return nil
	}
	ver, _ := EtcdVersionFromOptionsString(opts.String())
	return ver
}

func EtcdVersionFromOptionsString(opts string) (*semver.Version, error) {
	// TODO: Use proto.GetExtention when gogo/protobuf is usable with protoreflect
	msgs := []string{"[versionpb.etcd_version_msg]:", "[versionpb.etcd_version_field]:", "[versionpb.etcd_version_enum]:", "[versionpb.etcd_version_enum_value]:"}
	var end, index int
	for _, msg := range msgs {
		index = strings.Index(opts, msg)
		end = index + len(msg)
		if index != -1 {
			break
		}
	}
	if index == -1 {
		return nil, nil
	}
	var verStr string
	_, err := fmt.Sscanf(opts[end:], "%q", &verStr)
	if err != nil {
		return nil, err
	}
	if strings.Count(verStr, ".") == 1 {
		verStr = verStr + ".0"
	}
	ver, err := semver.NewVersion(verStr)
	if err != nil {
		return nil, err
	}
	return ver, nil
}
