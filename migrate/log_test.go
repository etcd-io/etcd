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
