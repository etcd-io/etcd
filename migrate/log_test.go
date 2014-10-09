package migrate

import (
	"reflect"
	"testing"
	"time"
)

func TestNewCommand(t *testing.T) {
	entries, err := ReadLogFile("fixtures/cmdlog")
	if err != nil {
		t.Errorf("read log file error: %v", err)
	}

	tests := []interface{}{
		&JoinCommand{2, 2, "1.local", "http://127.0.0.1:7001", "http://127.0.0.1:4001"},
		&SetClusterConfigCommand{&ClusterConfig{9, 1800.0, 5.0}},
		&NOPCommand{},
		&RemoveCommand{"alice"},
		&CompareAndDeleteCommand{"foo", "baz", 9},
		&CompareAndSwapCommand{"foo", "bar", time.Unix(0, 0), "baz", 9},
		&CreateCommand{"foo", "bar", time.Unix(0, 0), true, true},
		&DeleteCommand{"foo", true, true},
		&SetCommand{"foo", "bar", time.Unix(0, 0), true},
		&SyncCommand{time.Unix(0, 0)},
		&UpdateCommand{"foo", "bar", time.Unix(0, 0)},
		&DefaultLeaveCommand{"alice"},
		&DefaultJoinCommand{"alice", ""},
	}

	for i, e := range entries {
		cmd, err := NewCommand(e.GetCommandName(), e.GetCommand())
		if err != nil {
			t.Errorf("#%d: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(cmd, tests[i]) {
			t.Errorf("#%d: cmd = %+v, want %+v", i, cmd, tests[i])
		}
	}
}
