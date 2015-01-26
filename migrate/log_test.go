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

package migrate

import (
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestNewCommand(t *testing.T) {
	entries, err := DecodeLog4FromFile("fixtures/cmdlog")
	if err != nil {
		t.Errorf("read log file error: %v", err)
	}

	zeroTime, err := time.Parse(time.RFC3339, "1969-12-31T16:00:00-08:00")
	if err != nil {
		t.Errorf("couldn't create time: %v", err)
	}

	m := NewMember("alice", []url.URL{{Scheme: "http", Host: "127.0.0.1:7001"}}, etcdDefaultClusterName)
	m.ClientURLs = []string{"http://127.0.0.1:4001"}

	tests := []interface{}{
		&JoinCommand{"alice", "http://127.0.0.1:7001", "http://127.0.0.1:4001", *m},
		&NOPCommand{},
		&NOPCommand{},
		&RemoveCommand{"alice", 0xe52ada62956ff923},
		&CompareAndDeleteCommand{"foo", "baz", 9},
		&CompareAndSwapCommand{"foo", "bar", zeroTime, "baz", 9},
		&CreateCommand{"foo", "bar", zeroTime, true, true},
		&DeleteCommand{"foo", true, true},
		&SetCommand{"foo", "bar", zeroTime, true},
		&SyncCommand{zeroTime},
		&UpdateCommand{"foo", "bar", zeroTime},
	}

	raftMap := make(map[string]uint64)
	for i, test := range tests {
		e := entries[i]
		cmd, err := NewCommand4(e.GetCommandName(), e.GetCommand(), raftMap)
		if err != nil {
			t.Errorf("#%d: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(cmd, test) {
			if i == 5 {
				fmt.Println(cmd.(*CompareAndSwapCommand).ExpireTime.Location())
			}
			t.Errorf("#%d: cmd = %+v, want %+v", i, cmd, test)
		}
	}
}
