// Copyright 2015 CoreOS, Inc.
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
	"reflect"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
)

func TestEntry(t *testing.T) {
	tests := []raftpb.Entry{
		{},
		{Term: 1, Index: 1},
		{Term: 1, Index: 1, Data: []byte("some data")},
	}
	for i, tt := range tests {
		b := &bytes.Buffer{}
		if err := writeEntryTo(b, &tt); err != nil {
			t.Errorf("#%d: unexpected write ents error: %v", i, err)
			continue
		}
		var ent raftpb.Entry
		if err := readEntryFrom(b, &ent); err != nil {
			t.Errorf("#%d: unexpected read ents error: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(ent, tt) {
			t.Errorf("#%d: ent = %+v, want %+v", i, ent, tt)
		}
	}
}
