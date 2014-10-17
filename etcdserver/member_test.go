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
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.8:2379"}}, nil), 11240395089494390470},
		{newMember("mem1", []url.URL{{Scheme: "http", Host: "10.0.0.1:2379"}}, timeParse("1984-12-23T15:04:05Z")), 5483967913615174889},
	}
	for i, tt := range tests {
		if tt.mem.ID != tt.id {
			t.Errorf("#%d: mem.ID = %v, want %v", i, tt.mem.ID, tt.id)
		}
	}
}
