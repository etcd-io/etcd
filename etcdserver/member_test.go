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
		id  int64
	}{
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.8:2379"}}, nil), 7206348984215161146},
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}}, timeParse("1984-12-23T15:04:05Z")), 5483967913615174889},
	}
	for i, tt := range tests {
		if tt.mem.ID != tt.id {
			t.Errorf("#%d: mem.ID = %v, want %v", i, tt.mem.ID, tt.id)
		}
	}
}
