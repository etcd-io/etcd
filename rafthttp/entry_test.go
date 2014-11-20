/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package rafthttp

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
)

func TestEntsWriteAndRead(t *testing.T) {
	tests := [][]raftpb.Entry{
		{
			{},
		},
		{
			{Term: 1, Index: 1},
		},
		{
			{Term: 1, Index: 1},
			{Term: 1, Index: 2},
			{Term: 1, Index: 3},
		},
		{
			{Term: 1, Index: 1, Data: []byte("some data")},
			{Term: 1, Index: 2, Data: []byte("some data")},
			{Term: 1, Index: 3, Data: []byte("some data")},
		},
	}
	for i, tt := range tests {
		b := &bytes.Buffer{}
		ew := &entryWriter{w: b}
		if err := ew.writeEntries(tt); err != nil {
			t.Errorf("#%d: unexpected write ents error: %v", i, err)
			continue
		}
		er := &entryReader{r: b}
		ents, err := er.readEntries()
		if err != nil {
			t.Errorf("#%d: unexpected read ents error: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(ents, tt) {
			t.Errorf("#%d: ents = %+v, want %+v", i, ents, tt)
		}
	}
}
