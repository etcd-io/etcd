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

package osutil

import (
	"os"
	"reflect"
	"testing"
)

func TestUnsetenv(t *testing.T) {
	tests := []string{
		"data",
		"space data",
		"equal=data",
	}
	for i, tt := range tests {
		key := "ETCD_UNSETENV_TEST"
		if os.Getenv(key) != "" {
			t.Fatalf("#%d: cannot get empty %s", i, key)
		}
		env := os.Environ()
		if err := os.Setenv(key, tt); err != nil {
			t.Fatalf("#%d: cannot set %s: %v", i, key, err)
		}
		if err := Unsetenv(key); err != nil {
			t.Errorf("#%d: unsetenv %s error: %v", i, key, err)
		}
		if g := os.Environ(); !reflect.DeepEqual(g, env) {
			t.Errorf("#%d: env = %+v, want %+v", i, g, env)
		}
	}
}
