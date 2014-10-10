package etcdserver

import (
	"net/url"
	"testing"
	"time"
)

func timeParse(value string) *time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestMemberTime(t *testing.T) {
	tests := []struct {
		mem *Member
		id  uint64
	}{
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.8:2379"}}, nil), 14544069596553697298},
		// Same ID, different name (names shouldn't matter)
		{newMember("memfoo", []url.URL{{Scheme: "http", Host: "10.0.0.8:2379"}}, nil), 14544069596553697298},
		// Same ID, different Time
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.8:2379"}}, timeParse("1984-12-23T15:04:05Z")), 2448790162483548276},
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}}, timeParse("1984-12-23T15:04:05Z")), 1466075294948436910},
		// Order shouldn't matter
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}, {Scheme: "http", Host: "10.0.0.2:2379"}}, nil), 16552244735972308939},
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.2:2379"}, {Scheme: "http", Host: "10.0.0.1:2379"}}, nil), 16552244735972308939},
	}
	for i, tt := range tests {
		if tt.mem.ID != tt.id {
			t.Errorf("#%d: mem.ID = %v, want %v", i, tt.mem.ID, tt.id)
		}
	}
}
