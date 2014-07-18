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

func (p *participant) Set(key string, dir bool, value string, expireTime time.Time) (*store.Event, error) {
	set := &cmd{Type: "set", Key: key, Dir: dir, Value: value, Time: expireTime}
	return p.do(set)
}

func (p *participant) Create(key string, dir bool, value string, expireTime time.Time, unique bool) (*store.Event, error) {
	create := &cmd{Type: "create", Key: key, Dir: dir, Value: value, Time: expireTime, Unique: unique}
	return p.do(create)
}

func (p *participant) Update(key string, value string, expireTime time.Time) (*store.Event, error) {
	update := &cmd{Type: "update", Key: key, Value: value, Time: expireTime}
	return p.do(update)
}

func (p *participant) CAS(key, value, prevValue string, prevIndex uint64, expireTime time.Time) (*store.Event, error) {
	cas := &cmd{Type: "cas", Key: key, Value: value, PrevValue: prevValue, PrevIndex: prevIndex, Time: expireTime}
	return p.do(cas)
}

func (p *participant) Delete(key string, dir, recursive bool) (*store.Event, error) {
	d := &cmd{Type: "delete", Key: key, Dir: dir, Recursive: recursive}
	return p.do(d)
}

func (p *participant) CAD(key string, prevValue string, prevIndex uint64) (*store.Event, error) {
	cad := &cmd{Type: "cad", Key: key, PrevValue: prevValue, PrevIndex: prevIndex}
	return p.do(cad)
}

func (p *participant) do(c *cmd) (*store.Event, error) {
	data, err := json.Marshal(c)
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

	switch t := (<-pp.ret).(type) {
	case *store.Event:
		return t, nil
	case error:
		return nil, t
	default:
		panic("server.do: unexpected return type")
	}
}
