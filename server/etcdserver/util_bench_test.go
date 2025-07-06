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

package etcdserver

import (
	"errors"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"testing"
	"time"
)

func BenchmarkWarnOfExpensiveRequestNoLog(b *testing.B) {
	m := &raftpb.Message{
		Type:    0,
		To:      0,
		From:    1,
		Term:    2,
		LogTerm: 3,
		Index:   0,
		Entries: []raftpb.Entry{
			{
				Term:  0,
				Index: 0,
				Type:  0,
				Data:  make([]byte, 1024),
			},
		},
		Commit:     0,
		Snapshot:   raftpb.Snapshot{},
		Reject:     false,
		RejectHint: 0,
		Context:    nil,
	}
	err := errors.New("benchmarking warn of expensive request")
	for n := 0; n < b.N; n++ {
		warnOfExpensiveRequest(testLogger, time.Second, time.Now(), nil, m, err)
	}
}
