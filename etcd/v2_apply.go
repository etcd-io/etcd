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
	"encoding/json"
	"fmt"
	"log"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

func (p *participant) v2apply(index int64, ent raft.Entry) {
	var ret interface{}
	var e *store.Event
	var err error

	cmd := new(cmd)
	if err := json.Unmarshal(ent.Data, cmd); err != nil {
		log.Printf("participant.store.apply id=%x decodeErr=\"%v\"\n", p.id, err)
		return
	}

	switch cmd.Type {
	case "set":
		e, err = p.Store.Set(cmd.Key, cmd.Dir, cmd.Value, cmd.Time)
	case "update":
		e, err = p.Store.Update(cmd.Key, cmd.Value, cmd.Time)
	case "create", "unique":
		e, err = p.Store.Create(cmd.Key, cmd.Dir, cmd.Value, cmd.Unique, cmd.Time)
	case "delete":
		e, err = p.Store.Delete(cmd.Key, cmd.Dir, cmd.Recursive)
	case "cad":
		e, err = p.Store.CompareAndDelete(cmd.Key, cmd.PrevValue, cmd.PrevIndex)
	case "cas":
		e, err = p.Store.CompareAndSwap(cmd.Key, cmd.PrevValue, cmd.PrevIndex, cmd.Value, cmd.Time)
	case "sync":
		p.Store.DeleteExpiredKeys(cmd.Time)
		return
	default:
		log.Printf("participant.store.apply id=%x err=\"unexpected command type %s\"\n", p.id, cmd.Type)
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
