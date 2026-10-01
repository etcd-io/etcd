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
	"bytes"
	"math"
	"math/rand"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.etcd.io/raft/v3/raftpb"
)

// TestEntryDescriptorMatchesAppendEntry fails when raftpb.Entry gains, loses or
// changes a field, because appendEntry writes exactly these four and would
// silently drop anything else.
func TestEntryDescriptorMatchesAppendEntry(t *testing.T) {
	want := map[protowire.Number]protoreflect.Kind{
		1: protoreflect.EnumKind,
		2: protoreflect.Uint64Kind,
		3: protoreflect.Uint64Kind,
		4: protoreflect.BytesKind,
	}
	fields := (&raftpb.Entry{}).ProtoReflect().Descriptor().Fields()
	if fields.Len() != len(want) {
		t.Fatalf("raftpb.Entry has %d fields, appendEntry encodes %d", fields.Len(), len(want))
	}
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		kind, ok := want[fd.Number()]
		if !ok || fd.Kind() != kind || fd.Cardinality() != protoreflect.Optional || !fd.HasPresence() {
			t.Fatalf("field %d (%s): kind %v, cardinality %v, presence %v not handled by appendEntry", fd.Number(), fd.Name(), fd.Kind(), fd.Cardinality(), fd.HasPresence())
		}
	}
}

func TestAppendEntryMatchesProto(t *testing.T) {
	var unknown []byte
	unknown = protowire.AppendTag(unknown, 99, protowire.VarintType)
	unknown = protowire.AppendVarint(unknown, 7)

	cases := []*raftpb.Entry{
		{},
		{Data: []byte{}},
		{Type: raftpb.EntryNormal.Enum()},
		{Type: raftpb.EntryConfChangeV2.Enum(), Term: new(uint64(0)), Index: new(uint64(0))},
		{Type: raftpb.EntryType(-1).Enum(), Term: new(uint64(math.MaxUint64)), Index: new(uint64(1)), Data: []byte("x")},
		{Term: new(uint64(5)), Index: new(uint64(300)), Data: bytes.Repeat([]byte{0xab}, 1<<20)},
	}
	withUnknown := &raftpb.Entry{Term: new(uint64(1)), Index: new(uint64(2)), Data: []byte("u")}
	withUnknown.ProtoReflect().SetUnknown(unknown)
	cases = append(cases, withUnknown)

	r := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		e := &raftpb.Entry{}
		if r.Intn(2) == 0 {
			e.Type = raftpb.EntryType(r.Intn(3)).Enum()
		}
		if r.Intn(2) == 0 {
			e.Term = new(r.Uint64() >> uint(r.Intn(64)))
		}
		if r.Intn(2) == 0 {
			e.Index = new(r.Uint64() >> uint(r.Intn(64)))
		}
		if r.Intn(3) != 0 {
			e.Data = make([]byte, r.Intn(4096))
			r.Read(e.Data)
		}
		cases = append(cases, e)
	}

	for i, e := range cases {
		want, err := proto.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		prefix := []byte("prefix")
		got := appendEntry(append([]byte(nil), prefix...), e)
		if !bytes.Equal(got[:len(prefix)], prefix) || !bytes.Equal(got[len(prefix):], want) {
			t.Fatalf("case %d: appendEntry differs from proto.Marshal\n got %x\nwant %x", i, got[len(prefix):], want)
		}
		var back raftpb.Entry
		if err := proto.Unmarshal(got[len(prefix):], &back); err != nil || !proto.Equal(&back, e) {
			t.Fatalf("case %d: round trip failed: %v", i, err)
		}
	}
}
