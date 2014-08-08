/*
Copyright 2014 CoreOS Inc.

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

package etcd

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/store"
)

const (
	stset = iota
	stcreate
	stupdate
	stcas
	stdelete
	stcad
	stqget
	stsync
)

func (p *participant) Set(key string, dir bool, value string, expireTime time.Time) (*store.Event, error) {
	set := &Cmd{Type: stset, Key: key, Dir: &dir, Value: &value, Time: mustMarshalTime(&expireTime)}
	return p.do(set)
}

func (p *participant) Create(key string, dir bool, value string, expireTime time.Time, unique bool) (*store.Event, error) {
	create := &Cmd{Type: stcreate, Key: key, Dir: &dir, Value: &value, Time: mustMarshalTime(&expireTime), Unique: &unique}
	return p.do(create)
}

func (p *participant) Update(key string, value string, expireTime time.Time) (*store.Event, error) {
	update := &Cmd{Type: stupdate, Key: key, Value: &value, Time: mustMarshalTime(&expireTime)}
	return p.do(update)
}

func (p *participant) CAS(key, value, prevValue string, prevIndex uint64, expireTime time.Time) (*store.Event, error) {
	cas := &Cmd{Type: stcas, Key: key, Value: &value, PrevValue: &prevValue, PrevIndex: &prevIndex, Time: mustMarshalTime(&expireTime)}
	return p.do(cas)
}

func (p *participant) Delete(key string, dir, recursive bool) (*store.Event, error) {
	d := &Cmd{Type: stdelete, Key: key, Dir: &dir, Recursive: &recursive}
	return p.do(d)
}

func (p *participant) CAD(key string, prevValue string, prevIndex uint64) (*store.Event, error) {
	cad := &Cmd{Type: stcad, Key: key, PrevValue: &prevValue, PrevIndex: &prevIndex}
	return p.do(cad)
}
func (p *participant) QuorumGet(key string, recursive, sorted bool) (*store.Event, error) {
	get := &Cmd{Type: stqget, Key: key, Recursive: &recursive, Sorted: &sorted}
	return p.do(get)
}

func (p *participant) do(c *Cmd) (*store.Event, error) {
	data, err := c.Marshal()
	if err != nil {
		panic(err)
	}

	pp := v2Proposal{
		data: data,
		ret:  make(chan interface{}, 1),
	}

	select {
	case p.proposal <- pp:
	default:
		return nil, fmt.Errorf("unable to send out the proposal")
	}

	var ret interface{}
	select {
	case ret = <-pp.ret:
	case <-p.stopNotifyc:
		return nil, fmt.Errorf("stop serving")
	}

	switch t := ret.(type) {
	case *store.Event:
		return t, nil
	case error:
		return nil, t
	default:
		panic("server.do: unexpected return type")
	}
}
