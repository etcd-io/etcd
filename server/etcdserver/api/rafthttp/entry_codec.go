// Copyright 2026 The etcd Authors
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

package rafthttp

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"go.etcd.io/raft/v3/raftpb"
)

// entryMarshal skips the required-field check. raftpb has no required fields.
var entryMarshal = proto.MarshalOptions{AllowPartial: true}

// appendEntry appends the wire encoding of e to b. The output is byte-for-byte
// what proto.Marshal produces for e (fields in field-number order, unset
// optional fields omitted, unknown fields last), see TestAppendEntryMatchesProto.
// TestEntryDescriptorMatchesAppendEntry pins the fields this function knows.
//
// raftpb.Entry is the hottest message on the leader: the msgappv2 stream writer
// encodes every replicated entry once per follower, and the reflection-based
// marshaller of google.golang.org/protobuf spends most of the writer's CPU there.
func appendEntry(b []byte, e *raftpb.Entry) []byte {
	if e == nil {
		return b
	}
	if u := e.ProtoReflect().GetUnknown(); len(u) > 0 {
		out, err := entryMarshal.MarshalAppend(b, e)
		if err != nil {
			panic(fmt.Sprintf("marshal should never fail (%v)", err))
		}
		return out
	}
	if e.Type != nil {
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(int64(*e.Type)))
	}
	if e.Term != nil {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, *e.Term)
	}
	if e.Index != nil {
		b = protowire.AppendTag(b, 3, protowire.VarintType)
		b = protowire.AppendVarint(b, *e.Index)
	}
	if e.Data != nil {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendBytes(b, e.Data)
	}
	return b
}
