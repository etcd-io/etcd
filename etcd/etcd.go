package etcd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/store"
)

const (
	defaultHeartbeat = 1
	defaultElection  = 5

	defaultTickDuration = time.Millisecond * 100

	v2machineKVPrefix = "/_etcd/machines"
	v2Prefix          = "/v2/keys"
	v2machinePrefix   = "/v2/machines"
	v2LeaderPrefix    = "/v2/leader"

	raftPrefix = "/raft"
)

type Server struct {
	config *config.Config

	id           int
	pubAddr      string
	nodes        map[string]bool
	tickDuration time.Duration

	proposal chan v2Proposal
	node     *v2Raft
	t        *transporter

	store.Store

	stop chan struct{}

	http.Handler
}

func New(c *config.Config, id int) *Server {
	if err := c.Sanitize(); err != nil {
		log.Fatalf("failed sanitizing configuration: %v", err)
	}

	s := &Server{
		config:       c,
		id:           id,
		pubAddr:      c.Addr,
		nodes:        make(map[string]bool),
		tickDuration: defaultTickDuration,
		proposal:     make(chan v2Proposal),
		node: &v2Raft{
			Node:   raft.New(id, defaultHeartbeat, defaultElection),
			result: make(map[wait]chan interface{}),
		},
		t: newTransporter(),

		Store: store.New(),

		stop: make(chan struct{}),
	}

	for _, seed := range c.Peers {
		s.nodes[seed] = true
	}

	m := http.NewServeMux()
	//m.Handle("/HEAD", handlerErr(s.serveHead))
	m.Handle(v2Prefix+"/", handlerErr(s.serveValue))
	m.Handle("/raft", s.t)
	m.Handle(v2machinePrefix, handlerErr(s.serveMachines))
	m.Handle(v2LeaderPrefix, handlerErr(s.serveLeader))
	s.Handler = m
	return s
}

func (s *Server) SetTick(d time.Duration) {
	s.tickDuration = d
}

func (s *Server) Run() {
	if len(s.config.Peers) == 0 {
		s.Bootstrap()
	} else {
		s.Join()
	}
}

func (s *Server) Stop() {
	close(s.stop)
	s.t.stop()
}

func (s *Server) Bootstrap() {
	log.Println("starting a bootstrap node")
	s.node.Campaign()
	s.node.Add(s.id, s.pubAddr)
	s.apply(s.node.Next())
	s.run()
}

func (s *Server) Join() {
	log.Println("joining cluster via peers", s.config.Peers)
	d, err := json.Marshal(&raft.Config{s.id, s.pubAddr})
	if err != nil {
		panic(err)
	}

	b, err := json.Marshal(&raft.Message{From: s.id, Type: 2, Entries: []raft.Entry{{Type: 1, Data: d}}})
	if err != nil {
		panic(err)
	}

	for seed := range s.nodes {
		if err := s.t.send(seed+raftPrefix, b); err != nil {
			log.Println(err)
			continue
		}
		// todo(xiangli) WAIT for join to be committed or retry...
		break
	}
	s.run()
}

func (s *Server) run() {
	node := s.node
	recv := s.t.recv
	ticker := time.NewTicker(s.tickDuration)
	v2SyncTicker := time.NewTicker(time.Millisecond * 500)

	var proposal chan v2Proposal
	for {
		if node.HasLeader() {
			proposal = s.proposal
		} else {
			proposal = nil
		}
		select {
		case p := <-proposal:
			node.Propose(p)
		case msg := <-recv:
			node.Step(*msg)
		case <-ticker.C:
			node.Tick()
		case <-v2SyncTicker.C:
			node.Sync()
		case <-s.stop:
			log.Printf("Node: %d stopped\n", s.id)
			return
		}
		s.apply(node.Next())
		s.send(node.Msgs())
	}
}

func (s *Server) apply(ents []raft.Entry) {
	offset := s.node.Applied() - len(ents) + 1
	for i, ent := range ents {
		switch ent.Type {
		// expose raft entry type
		case raft.Normal:
			if len(ent.Data) == 0 {
				continue
			}
			s.v2apply(offset+i, ent)
		case raft.AddNode:
			cfg := new(raft.Config)
			if err := json.Unmarshal(ent.Data, cfg); err != nil {
				log.Println(err)
				break
			}
			if err := s.t.set(cfg.NodeId, cfg.Addr); err != nil {
				log.Println(err)
				break
			}
			log.Printf("Add Node %x %v\n", cfg.NodeId, cfg.Addr)
			s.nodes[cfg.Addr] = true
			p := path.Join(v2machineKVPrefix, fmt.Sprint(cfg.NodeId))
			s.Store.Set(p, false, cfg.Addr, store.Permanent)
		default:
			panic("unimplemented")
		}
	}
}

func (s *Server) send(msgs []raft.Message) {
	for i := range msgs {
		data, err := json.Marshal(msgs[i])
		if err != nil {
			// todo(xiangli): error handling
			log.Fatal(err)
		}
		// todo(xiangli): reuse routines and limit the number of sending routines
		// sync.Pool?
		go func(i int) {
			var err error
			if err = s.t.sendTo(msgs[i].To, data); err == nil {
				return
			}
			if err == errUnknownNode {
				err = s.fetchAddr(msgs[i].To)
			}
			if err == nil {
				err = s.t.sendTo(msgs[i].To, data)
			}
			if err != nil {
				log.Println(err)
			}
		}(i)
	}
}

func (s *Server) fetchAddr(nodeId int) error {
	for seed := range s.nodes {
		if err := s.t.fetchAddr(seed, nodeId); err == nil {
			return nil
		}
	}
	return fmt.Errorf("cannot fetch the address of node %d", nodeId)
}
