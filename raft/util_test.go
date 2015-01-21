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

package raft

import (
	"strings"
	"testing"

	pb "github.com/coreos/etcd/raft/raftpb"
)

var testFormatter EntryFormatter = func(data []byte) string {
	return strings.ToUpper(string(data))
}

func TestDescribeEntry(t *testing.T) {
	entry := pb.Entry{
		Term:  1,
		Index: 2,
		Type:  pb.EntryNormal,
		Data:  []byte("hello\x00world"),
	}

	defaultFormatted := DescribeEntry(entry)
	if defaultFormatted != "1/2 EntryNormal \"hello\\x00world\"" {
		t.Errorf("unexpected default output: %s", defaultFormatted)
	}

	customFormatted := testFormatter.DescribeEntry(entry)
	if customFormatted != "1/2 EntryNormal HELLO\x00WORLD" {
		t.Errorf("unexpected custom output: %s", customFormatted)
	}
}
