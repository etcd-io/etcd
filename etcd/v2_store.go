package etcd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreos/etcd/store"
)

type cmd struct {
	Type      string
	Key       string
	Value     string
	PrevValue string
	PrevIndex uint64
	Dir       bool
	Recursive bool
	Unique    bool
	Time      time.Time
}

func (s *Server) Set(key string, dir bool, value string, expireTime time.Time) (*store.Event, error) {
	set := &cmd{Type: "set", Key: key, Dir: dir, Value: value, Time: expireTime}
	return s.do(set)
}

func (s *Server) Create(key string, dir bool, value string, expireTime time.Time, unique bool) (*store.Event, error) {
	create := &cmd{Type: "create", Key: key, Dir: dir, Value: value, Time: expireTime, Unique: unique}
	return s.do(create)
}

func (s *Server) Update(key string, value string, expireTime time.Time) (*store.Event, error) {
	update := &cmd{Type: "update", Key: key, Value: value, Time: expireTime}
	return s.do(update)
}

func (s *Server) CAS(key, value, prevValue string, prevIndex uint64, expireTime time.Time) (*store.Event, error) {
	cas := &cmd{Type: "cas", Key: key, Value: value, PrevValue: prevValue, PrevIndex: prevIndex, Time: expireTime}
	return s.do(cas)
}

func (s *Server) Delete(key string, dir, recursive bool) (*store.Event, error) {
	d := &cmd{Type: "delete", Key: key, Dir: dir, Recursive: recursive}
	return s.do(d)
}

func (s *Server) CAD(key string, prevValue string, prevIndex uint64) (*store.Event, error) {
	cad := &cmd{Type: "cad", Key: key, PrevValue: prevValue, PrevIndex: prevIndex}
	return s.do(cad)
}

func (s *Server) do(c *cmd) (*store.Event, error) {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	p := v2Proposal{
		data: data,
		ret:  make(chan interface{}, 1),
	}

	if s.mode != participant {
		return nil, raftStopErr
	}
	select {
	case s.proposal <- p:
	default:
		return nil, fmt.Errorf("unable to send out the proposal")
	}

	switch t := (<-p.ret).(type) {
	case *store.Event:
		return t, nil
	case error:
		return nil, t
	default:
		panic("server.do: unexpected return type")
	}
}
