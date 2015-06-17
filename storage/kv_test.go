package storage

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

type kv struct {
	k, v []byte
}

// TestWorkflow simulates the whole workflow that storage is used in normal
// etcd running, including key changes, compaction and restart.
func TestWorkflow(t *testing.T) {
	s := newStore("test")
	defer os.Remove("test")

	var lastrev int64
	var wkvs []kv
	for i := 0; i < 10; i++ {
		// regular compaction
		s.Compact(lastrev)

		// put 100 keys into the store in each round
		for k := 0; k < 100; k++ {
			key := fmt.Sprintf("bar_%03d_%03d", i, k)
			val := fmt.Sprintf("foo_%03d_%03d", i, k)
			s.Put([]byte(key), []byte(val))
			wkvs = append(wkvs, kv{k: []byte(key), v: []byte(val)})
		}

		// delete second-half keys in this round
		key := fmt.Sprintf("bar_%03d_050", i)
		end := fmt.Sprintf("bar_%03d_100", i)
		if n, _ := s.DeleteRange([]byte(key), []byte(end)); n != 50 {
			t.Errorf("#%d: delete number = %d, want 50", i, n)
		}
		wkvs = wkvs[:len(wkvs)-50]

		// check existing keys
		kvs, rev, err := s.Range([]byte("bar"), []byte("bas"), 0, 0)
		if err != nil {
			t.Errorf("#%d: range error (%v)", err)
		}
		for j, kv := range kvs {
			if !reflect.DeepEqual(kv.Key, wkvs[j].k) {
				t.Errorf("#%d: keys[%d] = %s, want %s", i, j, kv.Key, wkvs[j].k)
			}
			if !reflect.DeepEqual(kv.Value, wkvs[j].v) {
				t.Errorf("#%d: vals[%d] = %s, want %s", i, j, kv.Value, wkvs[j].v)
			}
		}
		lastrev = rev

		// the store is restarted and restored from the disk file
		s.Close()
		s = newStore("test")
		s.Restore()
	}
}
