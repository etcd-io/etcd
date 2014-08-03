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
	"log"
	"time"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

func (p *participant) v2apply(index int64, ent raft.Entry) {
	var ret interface{}
	var e *store.Event
	var err error

	var cmd Cmd
	if err := cmd.Unmarshal(ent.Data); err != nil {
		log.Printf("id=%x participant.store.apply decodeErr=\"%v\"\n", p.id, err)
		return
	}

	switch cmd.Type {
	case stset:
		e, err = p.Store.Set(cmd.Key, *cmd.Dir, *cmd.Value, mustUnmarshalTime(cmd.Time))
	case stupdate:
		e, err = p.Store.Update(cmd.Key, *cmd.Value, mustUnmarshalTime(cmd.Time))
	case stcreate:
		e, err = p.Store.Create(cmd.Key, *cmd.Dir, *cmd.Value, *cmd.Unique, mustUnmarshalTime(cmd.Time))
	case stdelete:
		e, err = p.Store.Delete(cmd.Key, *cmd.Dir, *cmd.Recursive)
	case stcad:
		e, err = p.Store.CompareAndDelete(cmd.Key, *cmd.PrevValue, *cmd.PrevIndex)
	case stcas:
		e, err = p.Store.CompareAndSwap(cmd.Key, *cmd.PrevValue, *cmd.PrevIndex, *cmd.Value, mustUnmarshalTime(cmd.Time))
	case stqget:
		e, err = p.Store.Get(cmd.Key, *cmd.Recursive, *cmd.Sorted)
	case stsync:
		p.Store.DeleteExpiredKeys(mustUnmarshalTime(cmd.Time))
		return
	default:
		log.Printf("id=%x participant.store.apply err=\"unexpected command type %s\"\n", p.id, cmd.Type)
	}

	if ent.Term > p.node.term {
		p.node.term = ent.Term
		for k, v := range p.node.result {
			if k.term < p.node.term {
				v <- fmt.Errorf("proposal lost due to leader election")
				delete(p.node.result, k)
			}
		}
	}

	w := wait{index, ent.Term}
	if p.node.result[w] == nil {
		return
	}

	if err != nil {
		ret = err
	} else {
		ret = e
	}
	p.node.result[w] <- ret
	delete(p.node.result, w)
}

func mustMarshalTime(t *time.Time) []byte {
	b, err := t.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return b
}

func mustUnmarshalTime(b []byte) time.Time {
	var time time.Time
	if err := time.UnmarshalBinary(b); err != nil {
		panic(err)
	}
	return time
}
