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

// Package metrics provides minimalist instrumentation for client side in
// pull-based monitoring.
package metrics

import (
	"expvar"
	"fmt"
	"net/http"
)

// Counter is a number that increases over time monotonically.
type Counter interface {
	Add(delta int64)
	String() string
}

// Gauge returns instantaneous value that is expected to fluctuate over time.
type Gauge interface {
	Set(value int64)
	String() string
}

// Group aggregates counters, gauges and sub-groups.
// It implements http.Handler, which exposes the registered metrics in JSON format.
type Group struct {
	m *expvar.Map
}

func NewGroup() *Group {
	return &Group{
		m: new(expvar.Map).Init(),
	}
}

func (g *Group) Counter(key string) Counter {
	c := &counter{
		i: new(expvar.Int),
	}
	g.m.Set(key, c)
	return c
}

func (g *Group) Gauge(key string) Gauge {
	gg := &gauge{
		i: new(expvar.Int),
	}
	g.m.Set(key, gg)
	return gg
}

func (g *Group) Group(key string) *Group {
	gg := NewGroup()
	g.m.Set(key, gg)
	return gg
}

func (g *Group) Clear() { g.m.Init() }

func (g *Group) String() string { return g.m.String() }

func (g *Group) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "%s\n", g)
}

func (g *Group) Handler() http.Handler { return g }

type counter struct {
	i *expvar.Int
}

func (c *counter) Add(delta int64) { c.i.Add(delta) }

func (c *counter) String() string { return c.i.String() }

type gauge struct {
	i *expvar.Int
}

func (g *gauge) Set(value int64) { g.i.Set(value) }

func (g *gauge) String() string { return g.i.String() }
