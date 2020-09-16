/*
Copyright 2019 The Pdd Authors.

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

package mvcc

import (
	"reflect"
	"testing"
)

func Test_pruneRevisions(t *testing.T) {
	type args struct {
		revisions []revision
		ro        RangeOptions
	}
	tests := []struct {
		name    string
		args    args
		wantRet []revision
	}{
		{
			name: "MaxModRevision",
			args: args{
				revisions: []revision{{main: 1}, {main: 2}},
				ro:        RangeOptions{MaxModRevision: 1},
			},
			wantRet: []revision{{main: 1}},
		},
		{
			name: "MinModRevision And MaxModRevision",
			args: args{
				revisions: []revision{{main: 1}, {main: 2}, {main: 32}, {main: 4}},
				ro:        RangeOptions{MinModRevision: 1, MaxModRevision: 3},
			},
			wantRet: []revision{{main: 1}, {main: 2}},
		},
		{
			name: "MinModRevision",
			args: args{
				revisions: []revision{{main: 1}, {main: 2}, {main: 3}},
				ro:        RangeOptions{MinModRevision: 3},
			},
			wantRet: []revision{{main: 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet := pruneRevisions(tt.args.revisions, rangeOpToPrunableFuncs(tt.args.ro))
			t.Log(gotRet)
			if !reflect.DeepEqual(gotRet, tt.wantRet) {
				t.Errorf("pruneRevisions() = %v, want %v", gotRet, tt.wantRet)
			}
		})
	}
}
