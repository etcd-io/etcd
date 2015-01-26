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

// TestPublish tests function Publish and related creation functions.
func TestPublish(t *testing.T) {
	defer reset()
	Publish("string", new(expvar.String))
	NewCounter("counter")
	NewGauge("gauge")
	NewMap("map")

	keys := []string{"counter", "gauge", "map", "string"}
	i := 0
	Do(func(kv expvar.KeyValue) {
		if kv.Key != keys[i] {
			t.Errorf("#%d: key = %s, want %s", i, kv.Key, keys[i])
		}
		i++
	})
}

func TestDuplicatePublish(t *testing.T) {
	defer reset()
	num1 := new(expvar.Int)
	num1.Set(10)
	Publish("number", num1)
	num2 := new(expvar.Int)
	num2.Set(20)
	Publish("number", num2)
	if g := Get("number").String(); g != "20" {
		t.Errorf("number str = %s, want %s", g, "20")
	}
}

// TestMap tests the basic usage of Map.
func TestMap(t *testing.T) {
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
