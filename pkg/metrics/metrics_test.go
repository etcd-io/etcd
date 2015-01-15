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

package metrics

import "testing"

// TestMetrics tests the basic usage of metrics group.
func TestMetrics(t *testing.T) {
	group := NewGroup()
	// use counter
	counter := group.Counter("Number")
	counter.Add(1234)
	// use sub-group
	subgroup := group.Group("shares")
	// use gauge
	subgroup.Gauge("alice").Set(30)
	subgroup.Gauge("bob").Set(50)

	wstr := `{"Number": 1234, "shares": {"alice": 30, "bob": 50}}`
	if str := group.String(); str != wstr {
		t.Errorf("group = %v, want %v", str, wstr)
	}

	// clear group
	group.Clear()
	if g := group.String(); g != "{}" {
		t.Errorf("group = %v, want {}", g)
	}
}
