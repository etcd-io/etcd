// Copyright 2019 The etcd Authors
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

package timeprefix

import (
	"sort"
	"testing"
	"time"
)

func TestPrefix(t *testing.T) {
	testCases := []struct {
		before    time.Duration
		drange    time.Duration
		date      string
		wantearly bool
		wantlate  bool
	}{
		{before: time.Hour * 24, drange: time.Hour * 24, date: Now(), wantearly: true},
		{before: time.Hour * 24, drange: time.Hour * 24, date: Past(time.Hour * 30)},
		{before: time.Hour * 24, drange: time.Hour * 24, date: Past((time.Hour * 48) - time.Minute - time.Second)},
		{before: time.Hour * 24, drange: time.Hour * 24, date: Past((time.Hour * 48) + time.Minute), wantlate: true},
	}

	for ti, tt := range testCases {
		pre := Prefixes(tt.before, tt.drange)
		pre = append(pre, tt.date)

		sort.Strings(pre)
		i := sort.StringSlice(pre).Search(tt.date)
		if tt.wantearly && i != 2 {
			t.Fatalf("%d: early date at index %d: %v", ti, i, pre)
		}
		if tt.wantlate && i != 0 {
			t.Fatalf("%d: late date at index %d: %v", ti, i, pre)
		}
	}
}
