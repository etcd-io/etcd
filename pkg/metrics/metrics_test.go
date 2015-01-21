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

import (
	"expvar"
	"testing"
)

// TestMetrics tests the basic usage of metrics.
func TestMetrics(t *testing.T) {
	m := &Map{Map: new(expvar.Map).Init()}

	c := m.NewCounter("number")
	c.Add()
	c.AddBy(10)
	if w := "11"; c.String() != w {
		t.Errorf("counter = %s, want %s", c, w)
	}

	g := m.NewGauge("price")
	g.Set(100)
	if w := "100"; g.String() != w {
		t.Errorf("gauge = %s, want %s", g, w)
	}

	if w := `{"number": 11, "price": 100}`; m.String() != w {
		t.Errorf("map = %s, want %s", m, w)
	}
	m.Delete("price")
	if w := `{"number": 11}`; m.String() != w {
		t.Errorf("map after deletion = %s, want %s", m, w)
	}
}
