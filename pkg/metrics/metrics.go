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

// Package metrics provides metrics view of variables which is exposed through
// expvar package.
//
// Naming conventions:
// 1. volatile path components should be kept as deep into the hierarchy as possible
// 2. each path component should have a clear and well-defined purpose
// 3. components.separated.with.dot, and put package prefix at the head
// 4. words_separated_with_underscore, and put clarifiers last, e.g., requests_total
package metrics

import (
	"bytes"
	"expvar"
	"fmt"
	"sort"
	"sync"
)

// Counter is a number that increases over time monotonically.
type Counter struct{ i *expvar.Int }

func (c *Counter) Add() { c.i.Add(1) }

func (c *Counter) AddBy(delta int64) { c.i.Add(delta) }

func (c *Counter) String() string { return c.i.String() }

// Gauge returns instantaneous value that is expected to fluctuate over time.
type Gauge struct{ i *expvar.Int }

func (g *Gauge) Set(value int64) { g.i.Set(value) }

func (g *Gauge) String() string { return g.i.String() }

type nilVar struct{}

func (v *nilVar) String() string { return "nil" }

// Map aggregates Counters and Gauges.
type Map struct{ *expvar.Map }

func (m *Map) NewCounter(key string) *Counter {
	c := &Counter{i: new(expvar.Int)}
	m.Set(key, c)
	return c
}

func (m *Map) NewGauge(key string) *Gauge {
	g := &Gauge{i: new(expvar.Int)}
	m.Set(key, g)
	return g
}

// TODO: remove the var from the map to avoid memory boom
func (m *Map) Delete(key string) { m.Set(key, &nilVar{}) }

// String returns JSON format string that represents the group.
// It does not print out nilVar.
func (m *Map) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "{")
	first := true
	m.Do(func(kv expvar.KeyValue) {
		v := kv.Value.String()
		if v == "nil" {
			return
		}
		if !first {
			fmt.Fprintf(&b, ", ")
		}
		fmt.Fprintf(&b, "%q: %v", kv.Key, v)
		first = false
	})
	fmt.Fprintf(&b, "}")
	return b.String()
}

// All published variables.
var (
	mutex   sync.RWMutex
	vars    = make(map[string]expvar.Var)
	varKeys []string // sorted
)

// Publish declares a named exported variable.
// If the name is already registered then this will overwrite the old one.
func Publish(name string, v expvar.Var) {
	mutex.Lock()
	defer mutex.Unlock()
	if _, existing := vars[name]; !existing {
		varKeys = append(varKeys, name)
	}
	sort.Strings(varKeys)
	vars[name] = v
	return
}

// Get retrieves a named exported variable.
func Get(name string) expvar.Var {
	mutex.RLock()
	defer mutex.RUnlock()
	return vars[name]
}

// Convenience functions for creating new exported variables.
func NewCounter(name string) *Counter {
	c := &Counter{i: new(expvar.Int)}
	Publish(name, c)
	return c
}

func NewGauge(name string) *Gauge {
	g := &Gauge{i: new(expvar.Int)}
	Publish(name, g)
	return g
}

func NewMap(name string) *Map {
	m := &Map{Map: new(expvar.Map).Init()}
	Publish(name, m)
	return m
}

// GetMap returns the map if it exists, or inits the given name map if it does
// not exist.
func GetMap(name string) *Map {
	v := Get(name)
	if v == nil {
		return NewMap(name)
	}
	return v.(*Map)
}

// Do calls f for each exported variable.
// The global variable map is locked during the iteration,
// but existing entries may be concurrently updated.
func Do(f func(expvar.KeyValue)) {
	mutex.RLock()
	defer mutex.RUnlock()
	for _, k := range varKeys {
		f(expvar.KeyValue{k, vars[k]})
	}
}

// for test only
func reset() {
	mutex.Lock()
	defer mutex.Unlock()
	vars = make(map[string]expvar.Var)
	varKeys = nil
}
