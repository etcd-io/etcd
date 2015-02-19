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

package starter

import (
	"flag"
	"reflect"
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		args  []string
		wvals map[string]string
	}{
		{
			[]string{"--name", "etcd", "--data-dir", "dir"},
			map[string]string{
				"name":     "etcd",
				"data-dir": "dir",
			},
		},
		{
			[]string{"--name=etcd", "--data-dir=dir"},
			map[string]string{
				"name":     "etcd",
				"data-dir": "dir",
			},
		},
		{
			[]string{"--version", "--name", "etcd"},
			map[string]string{
				"version": "true",
				"name":    "etcd",
			},
		},
		{
			[]string{"--version=true", "--name", "etcd"},
			map[string]string{
				"version": "true",
				"name":    "etcd",
			},
		},
	}
	for i, tt := range tests {
		fs, err := parseConfig(tt.args)
		if err != nil {
			t.Fatalf("#%d: unexpected parseConfig error: %v", i, err)
		}
		vals := make(map[string]string)
		fs.Visit(func(f *flag.Flag) {
			vals[f.Name] = f.Value.String()
		})
		if !reflect.DeepEqual(vals, tt.wvals) {
			t.Errorf("#%d: vals = %+v, want %+v", i, vals, tt.wvals)
		}
	}
}
