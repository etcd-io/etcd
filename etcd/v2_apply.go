package etcd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

func (s *Server) v2apply(index int, ent raft.Entry) {
	var ret interface{}
	var e *store.Event
	var err error

	cmd := new(cmd)
	if err := json.Unmarshal(ent.Data, cmd); err != nil {
		log.Println("v2apply.decode:", err)
		return
	}

	switch cmd.Type {
	case "set":
		e, err = s.Store.Set(cmd.Key, cmd.Dir, cmd.Value, cmd.Time)
	case "update":
		e, err = s.Store.Update(cmd.Key, cmd.Value, cmd.Time)
	case "create", "unique":
		e, err = s.Store.Create(cmd.Key, cmd.Dir, cmd.Value, cmd.Unique, cmd.Time)
	case "delete":
		e, err = s.Store.Delete(cmd.Key, cmd.Dir, cmd.Recursive)
	case "cad":
		e, err = s.Store.CompareAndDelete(cmd.Key, cmd.PrevValue, cmd.PrevIndex)
	case "cas":
		e, err = s.Store.CompareAndSwap(cmd.Key, cmd.PrevValue, cmd.PrevIndex, cmd.Value, cmd.Time)
	case "sync":
		s.Store.DeleteExpiredKeys(cmd.Time)
		return
	default:
		log.Println("unexpected command type:", cmd.Type)
	}

	if ent.Term > s.node.term {
		s.node.term = ent.Term
		for k, v := range s.node.result {
			if k.term < s.node.term {
				v <- fmt.Errorf("proposal lost due to leader election")
				delete(s.node.result, k)
			}
		}
	}

	if s.node.result[wait{index, ent.Term}] == nil {
		return
	}

	if err != nil {
		ret = err
	} else {
		ret = e
	}
	s.node.result[wait{index, ent.Term}] <- ret
}
